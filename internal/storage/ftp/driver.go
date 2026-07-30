// Package ftp implements a compatibility-tier storage driver over FTP/FTPS.
// Prefer OpenList/rclone → WebDAV/local/S3 for production; this path is for
// vendors that only expose FTP and operators who refuse an extra proxy.
//
// Hot-path cost is reduced inside this package only: a small connection pool
// reuses logged-in sessions, and the working TLS mode is remembered so each
// request does not dual-dial explicit+implicit. serve/storagesvc stay unaware.
package ftp

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jlaffaye/ftp"

	"github.com/yixian-huang/imgli/internal/storage"
)

// Default pool knobs — kept internal (no admin UI) so the driver stays removable.
const (
	defaultMaxPool  = 4
	defaultIdleTTL  = 90 * time.Second
	defaultDialWait = 15 * time.Second
)

// tlsMode is how the control connection is secured. Remembered after first success.
type tlsMode int

const (
	tlsUnknown tlsMode = iota
	tlsPlain           // allow_insecure only
	tlsExplicit        // AUTH TLS after connect
	tlsImplicit        // TLS from first byte (often :990)
)

// Driver is an FTP/FTPS storage backend (compat tier).
// Connections are pooled under maxPool with idle eviction; ops that fail drop
// the connection instead of returning it to the pool.
type Driver struct {
	host          string
	port          int
	user          string
	pass          string
	prefix        string // remote base path without trailing slash
	allowInsecure bool
	disableEPSV   bool
	dialTimeout   time.Duration
	maxPool       int
	idleTTL       time.Duration

	mu   sync.Mutex
	pool []*pooledConn
	mode tlsMode // tlsUnknown until first successful dial
}

type pooledConn struct {
	c    *ftp.ServerConn
	last time.Time
}

// New builds a Driver from policy config.
//
// Required: host (or endpoint)
// Optional: port (default 21), username, password, prefix, allow_insecure,
// disable_epsv ("true" for some broken NATs).
//
// TLS: default FTPS — first dial probes explicit then implicit once, then reuses
// the working mode. allow_insecure=true uses plain FTP only.
func New(cfg map[string]string) (*Driver, error) {
	host := strings.TrimSpace(cfg["host"])
	if host == "" {
		ep := strings.TrimSpace(cfg["endpoint"])
		ep = strings.TrimPrefix(ep, "ftp://")
		ep = strings.TrimPrefix(ep, "ftps://")
		if i := strings.Index(ep, "/"); i >= 0 {
			ep = ep[:i]
		}
		host = ep
	}
	if host == "" {
		return nil, errors.New("ftp: host required")
	}
	port := 21
	if p := strings.TrimSpace(cfg["port"]); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil || n <= 0 || n > 65535 {
			return nil, errors.New("ftp: invalid port")
		}
		port = n
	} else if h, p, err := net.SplitHostPort(host); err == nil {
		host = h
		if n, err := strconv.Atoi(p); err == nil {
			port = n
		}
	}
	prefix := strings.Trim(strings.TrimSpace(cfg["prefix"]), "/")
	if prefix == "" {
		prefix = strings.Trim(strings.TrimSpace(cfg["root"]), "/")
	}
	return &Driver{
		host:          host,
		port:          port,
		user:          cfg["username"],
		pass:          cfg["password"],
		prefix:        prefix,
		allowInsecure: strings.EqualFold(strings.TrimSpace(cfg["allow_insecure"]), "true"),
		disableEPSV:   strings.EqualFold(strings.TrimSpace(cfg["disable_epsv"]), "true"),
		dialTimeout:   defaultDialWait,
		maxPool:       defaultMaxPool,
		idleTTL:       defaultIdleTTL,
		mode:          tlsUnknown,
	}, nil
}

func (d *Driver) Capabilities() storage.Caps {
	c, _ := storage.CapsForDriver("ftp")
	return c
}

func (d *Driver) remotePath(key string) string {
	key = strings.TrimLeft(key, "/")
	if d.prefix == "" {
		return key
	}
	return d.prefix + "/" + key
}

func (d *Driver) addr() string {
	return net.JoinHostPort(d.host, strconv.Itoa(d.port))
}

func (d *Driver) baseOpts() []ftp.DialOption {
	opts := []ftp.DialOption{ftp.DialWithTimeout(d.dialTimeout)}
	if d.disableEPSV {
		opts = append(opts, ftp.DialWithDisabledEPSV(true))
	}
	return opts
}

func (d *Driver) login(c *ftp.ServerConn) error {
	user := d.user
	if user == "" {
		user = "anonymous"
	}
	if err := c.Login(user, d.pass); err != nil {
		_ = c.Quit()
		return fmt.Errorf("ftp login: %w", err)
	}
	return nil
}

// dialMode opens one control connection for a known mode (no fallbacks).
func (d *Driver) dialMode(ctx context.Context, mode tlsMode) (*ftp.ServerConn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	opts := d.baseOpts()
	switch mode {
	case tlsPlain:
		// nothing
	case tlsExplicit:
		tlsCfg := &tls.Config{ServerName: d.host, MinVersion: tls.VersionTLS12}
		opts = append(opts, ftp.DialWithExplicitTLS(tlsCfg))
	case tlsImplicit:
		tlsCfg := &tls.Config{ServerName: d.host, MinVersion: tls.VersionTLS12}
		opts = append(opts, ftp.DialWithTLS(tlsCfg))
	default:
		return nil, errors.New("ftp: unknown tls mode")
	}
	c, err := ftp.Dial(d.addr(), opts...)
	if err != nil {
		return nil, err
	}
	if err := d.login(c); err != nil {
		return nil, err
	}
	return c, nil
}

// dialFresh establishes a connection. When mode is unknown, probes once and
// returns the working mode so the caller can remember it.
func (d *Driver) dialFresh(ctx context.Context, preferred tlsMode) (*ftp.ServerConn, tlsMode, error) {
	if d.allowInsecure {
		c, err := d.dialMode(ctx, tlsPlain)
		return c, tlsPlain, err
	}
	if preferred == tlsExplicit || preferred == tlsImplicit {
		c, err := d.dialMode(ctx, preferred)
		return c, preferred, err
	}
	// First connection only: try explicit then implicit (not on every request).
	c, err := d.dialMode(ctx, tlsExplicit)
	if err == nil {
		return c, tlsExplicit, nil
	}
	c2, err2 := d.dialMode(ctx, tlsImplicit)
	if err2 != nil {
		return nil, tlsUnknown, fmt.Errorf("ftp dial: explicit: %v; implicit: %w", err, err2)
	}
	return c2, tlsImplicit, nil
}

// acquire returns a logged-in connection (from pool or new dial).
func (d *Driver) acquire(ctx context.Context) (*ftp.ServerConn, error) {
	for {
		d.mu.Lock()
		// Drop expired idle conns from the end (LIFO).
		for len(d.pool) > 0 {
			i := len(d.pool) - 1
			p := d.pool[i]
			d.pool = d.pool[:i]
			if time.Since(p.last) > d.idleTTL {
				_ = p.c.Quit()
				continue
			}
			c := p.c
			d.mu.Unlock()
			if err := c.NoOp(); err != nil {
				_ = c.Quit()
				// try next pooled / dial
				continue
			}
			return c, nil
		}
		preferred := d.mode
		d.mu.Unlock()

		c, mode, err := d.dialFresh(ctx, preferred)
		if err != nil {
			return nil, fmt.Errorf("ftp dial: %w", err)
		}
		d.mu.Lock()
		if d.mode == tlsUnknown {
			d.mode = mode
		}
		d.mu.Unlock()
		return c, nil
	}
}

// release returns a healthy conn to the pool, or closes it.
func (d *Driver) release(c *ftp.ServerConn, bad bool) {
	if c == nil {
		return
	}
	if bad {
		_ = c.Quit()
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.pool) >= d.maxPool {
		_ = c.Quit()
		return
	}
	d.pool = append(d.pool, &pooledConn{c: c, last: time.Now()})
}

// withConn runs fn with a pooled or fresh connection; reuses on success.
// storage.ErrNotFound is not treated as a dead connection (object missing).
func (d *Driver) withConn(ctx context.Context, fn func(*ftp.ServerConn) error) error {
	c, err := d.acquire(ctx)
	if err != nil {
		return err
	}
	err = fn(c)
	bad := err != nil && !errors.Is(err, storage.ErrNotFound)
	d.release(c, bad)
	return err
}

// Close drains the pool (optional; not part of storage.Driver).
func (d *Driver) Close() {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, p := range d.pool {
		_ = p.c.Quit()
	}
	d.pool = nil
}

func (d *Driver) mkParents(c *ftp.ServerConn, remote string) error {
	dir := path.Dir(remote)
	if dir == "." || dir == "/" || dir == "" {
		return nil
	}
	parts := strings.Split(dir, "/")
	cur := ""
	for _, p := range parts {
		if p == "" {
			continue
		}
		if cur == "" {
			cur = p
		} else {
			cur = cur + "/" + p
		}
		_ = c.MakeDir(cur) // ignore exists
	}
	return nil
}

func (d *Driver) Put(ctx context.Context, key string, r io.Reader) error {
	var body io.Reader = r
	var buf []byte
	if _, ok := r.(io.ReadSeeker); !ok {
		var err error
		buf, err = io.ReadAll(r)
		if err != nil {
			return err
		}
		body = bytes.NewReader(buf)
	}
	remote := d.remotePath(key)
	return d.withConn(ctx, func(c *ftp.ServerConn) error {
		if err := d.mkParents(c, remote); err != nil {
			return err
		}
		if err := c.Stor(remote, body); err != nil {
			if buf == nil {
				if rs, ok := body.(io.ReadSeeker); ok {
					_, _ = rs.Seek(0, io.SeekStart)
				}
			} else {
				body = bytes.NewReader(buf)
			}
			_ = d.mkParents(c, remote)
			if err2 := c.Stor(remote, body); err2 != nil {
				return fmt.Errorf("ftp stor: %w", err2)
			}
		}
		return nil
	})
}

func (d *Driver) Open(ctx context.Context, key string) (io.ReadSeekCloser, error) {
	// Still buffer whole object: ServeContent needs Seek; FTP REST is uneven.
	// Pooling removes per-request dial/login; TTFB still includes full Retr.
	var data []byte
	err := d.withConn(ctx, func(c *ftp.ServerConn) error {
		resp, err := c.Retr(d.remotePath(key))
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "no such") ||
				strings.Contains(err.Error(), "550") {
				return storage.ErrNotFound
			}
			return err
		}
		defer resp.Close()
		data, err = io.ReadAll(resp)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &memRSC{r: bytes.NewReader(data)}, nil
}

func (d *Driver) Delete(ctx context.Context, key string) error {
	return d.withConn(ctx, func(c *ftp.ServerConn) error {
		err := c.Delete(d.remotePath(key))
		if err != nil {
			if strings.Contains(err.Error(), "550") {
				return nil
			}
			return err
		}
		return nil
	})
}

func (d *Driver) Exists(ctx context.Context, key string) (bool, error) {
	var ok bool
	err := d.withConn(ctx, func(c *ftp.ServerConn) error {
		_, err := c.FileSize(d.remotePath(key))
		if err != nil {
			// 550 / no such → missing; other errors drop the connection
			if strings.Contains(err.Error(), "550") ||
				strings.Contains(strings.ToLower(err.Error()), "no such") {
				ok = false
				return nil
			}
			return err
		}
		ok = true
		return nil
	})
	return ok, err
}

type memRSC struct {
	r *bytes.Reader
}

func (m *memRSC) Read(p []byte) (int, error)           { return m.r.Read(p) }
func (m *memRSC) Seek(off int64, wh int) (int64, error) { return m.r.Seek(off, wh) }
func (m *memRSC) Close() error                          { return nil }
