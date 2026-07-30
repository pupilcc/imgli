// Package ftp implements a compatibility-tier storage driver over FTP/FTPS.
// Prefer OpenList/rclone → WebDAV/local/S3 for production; this path is for
// vendors that only expose FTP and operators who refuse an extra proxy.
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

// Driver is an FTP/FTPS storage backend. Not safe for extreme concurrency;
// each operation dials (or reuses a short-lived connection under a mutex).
type Driver struct {
	host           string
	port           int
	user           string
	pass           string
	prefix         string // remote base path without trailing slash
	allowInsecure  bool
	disableEPSV    bool
	dialTimeout    time.Duration
	mu             sync.Mutex
}

// New builds a Driver from policy config.
//
// Required: host
// Optional: port (default 21), username, password, prefix, allow_insecure (plain FTP),
// disable_epsv ("true" for some broken NATs).
//
// TLS: by default uses explicit FTPS (STARTTLS-style via DialWithExplicitTLS when
// available) — we use DialWithTLS for implicit TLS on port 990-style, and for
// default port 21 we dial plain only when allow_insecure=true; otherwise we use
// DialWithExplicitTLS.
func New(cfg map[string]string) (*Driver, error) {
	host := strings.TrimSpace(cfg["host"])
	if host == "" {
		// Accept endpoint as host[:port] for familiarity.
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
		dialTimeout:   15 * time.Second,
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

func (d *Driver) dial(ctx context.Context) (*ftp.ServerConn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	addr := net.JoinHostPort(d.host, strconv.Itoa(d.port))
	opts := []ftp.DialOption{
		ftp.DialWithTimeout(d.dialTimeout),
	}
	if d.disableEPSV {
		opts = append(opts, ftp.DialWithDisabledEPSV(true))
	}
	var c *ftp.ServerConn
	var err error
	if d.allowInsecure {
		c, err = ftp.Dial(addr, opts...)
	} else {
		tlsCfg := &tls.Config{ServerName: d.host, MinVersion: tls.VersionTLS12}
		// Explicit TLS (AUTH TLS) on standard FTP ports; also works when server expects it after connect.
		opts = append(opts, ftp.DialWithExplicitTLS(tlsCfg))
		c, err = ftp.Dial(addr, opts...)
		if err != nil {
			// Fallback: implicit TLS (common on 990).
			opts2 := []ftp.DialOption{
				ftp.DialWithTimeout(d.dialTimeout),
				ftp.DialWithTLS(tlsCfg),
			}
			if d.disableEPSV {
				opts2 = append(opts2, ftp.DialWithDisabledEPSV(true))
			}
			c, err = ftp.Dial(addr, opts2...)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("ftp dial: %w", err)
	}
	user := d.user
	if user == "" {
		user = "anonymous"
	}
	if err := c.Login(user, d.pass); err != nil {
		_ = c.Quit()
		return nil, fmt.Errorf("ftp login: %w", err)
	}
	return c, nil
}

func (d *Driver) withConn(ctx context.Context, fn func(*ftp.ServerConn) error) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	c, err := d.dial(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = c.Quit() }()
	return fn(c)
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
	// Buffer if non-seeker so we can retry parent create.
	var body io.Reader = r
	var buf []byte
	if rs, ok := r.(io.ReadSeeker); ok {
		_ = rs
	} else {
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
			// recreate parents and retry once with buffer
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
	// FTP REST/seek is uneven; load into memory for Seek compatibility with serve.
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
			// idempotent delete
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
			ok = false
			return nil
		}
		ok = true
		return nil
	})
	return ok, err
}

type memRSC struct {
	r *bytes.Reader
}

func (m *memRSC) Read(p []byte) (int, error)         { return m.r.Read(p) }
func (m *memRSC) Seek(off int64, wh int) (int64, error) { return m.r.Seek(off, wh) }
func (m *memRSC) Close() error                         { return nil }
