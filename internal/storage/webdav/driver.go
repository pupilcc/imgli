// Package webdav 实现 WebDAV 存储驱动(HTTP Basic 认证,零额外依赖)。
package webdav

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/yixian-huang/imgli/internal/storage"
)

// Driver 是 WebDAV 存储驱动。
type Driver struct {
	baseURL  string
	username string
	password string
	Client   *http.Client
}

// New 从配置 map 构造 Driver。必填: endpoint(含 scheme 的完整基址)。
// 可选: username, password(可空,开放 WebDAV)。
func New(cfg map[string]string) (*Driver, error) {
	endpoint := strings.TrimSpace(cfg["endpoint"])
	if endpoint == "" {
		return nil, fmt.Errorf("webdav: 缺少必填配置 endpoint")
	}
	u, err := url.Parse(endpoint)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("webdav: endpoint 非法 URL")
	}
	// 拒 endpoint 内联 userinfo(user:pass@):net/http 会据此生成 Basic 凭据,但
	// DTO 只打码独立 password 字段,userinfo 明文会经列表/编辑回显(codex 终审)。
	if u.User != nil {
		return nil, fmt.Errorf("webdav: endpoint 不得含用户名密码,请用独立字段")
	}
	return &Driver{
		baseURL:  strings.TrimRight(endpoint, "/"),
		username: cfg["username"],
		password: cfg["password"],
	}, nil
}

func (d *Driver) httpClient() *http.Client {
	if d.Client != nil {
		return d.Client
	}
	return &http.Client{
		Timeout:       30 * time.Second,
		CheckRedirect: webdavCheckRedirect,
	}
}

// webdavCheckRedirect 控制跟随策略：
//   - 仅 GET 跟随（OpenList 网盘代理常 302 到预签名直链；直链上 HEAD 常 403）
//   - HEAD/PUT/DELETE/MKCOL 不跟随（HEAD 302 由 headSize/Exists 本地解释；写 3xx 不当成功）
//   - 跟随前去掉 Authorization，避免 Basic 泄漏到对象存储（含同 host 不同端口）
func webdavCheckRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return fmt.Errorf("webdav: stopped after %d redirects", len(via))
	}
	if via[0].Method != http.MethodGet {
		return http.ErrUseLastResponse
	}
	req.Header.Del("Authorization")
	return nil
}

// encodePath 按 / 分段 url.PathEscape,保留层级分隔。
func encodePath(key string) string {
	segs := strings.Split(key, "/")
	for i, s := range segs {
		segs[i] = url.PathEscape(s)
	}
	return strings.Join(segs, "/")
}

func (d *Driver) objURL(key string) string {
	return d.baseURL + "/" + encodePath(key)
}

// bodyLen 探测 body 长度以显式设 Content-Length。*os.File 与 Len() 类可测;
// 未知类型返回 -1(退回 http 默认,可能 chunked)。
func bodyLen(r io.Reader) int64 {
	switch v := r.(type) {
	case nil:
		return 0
	case *os.File:
		if fi, err := v.Stat(); err == nil {
			return fi.Size()
		}
	case interface{ Len() int }:
		return int64(v.Len())
	}
	return -1
}

func (d *Driver) newReq(ctx context.Context, method, key string, body io.Reader, contentLength int64) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, d.objURL(key), body)
	if err != nil {
		return nil, err
	}
	if d.username != "" {
		req.SetBasicAuth(d.username, d.password)
	}
	if contentLength >= 0 {
		req.ContentLength = contentLength
	}
	return req, nil
}

func (d *Driver) do(ctx context.Context, method, key string, body io.Reader, contentLength int64) (*http.Response, error) {
	req, err := d.newReq(ctx, method, key, body, contentLength)
	if err != nil {
		return nil, err
	}
	return d.httpClient().Do(req)
}

// getRange 发带 Range 的 GET,供 rangeReadSeekCloser 惰性开流。
func (d *Driver) getRange(ctx context.Context, key string, offset int64) (*http.Response, error) {
	req, err := d.newReq(ctx, http.MethodGet, key, nil, -1)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	return d.httpClient().Do(req)
}

// mkcolParents 对 key 各级祖先集合逐个 MKCOL(不含文件名最后一段)。
// 2xx / 405(已存在) / 301 视为就绪;401/403 等真错返回。
func (d *Driver) mkcolParents(ctx context.Context, key string) error {
	parts := strings.Split(key, "/")
	if len(parts) <= 1 {
		return nil
	}
	// 前缀: 2026, 2026/07, 2026/07/19(不含最后文件名)
	for i := 1; i < len(parts); i++ {
		prefix := strings.Join(parts[:i], "/")
		resp, err := d.do(ctx, "MKCOL", prefix, nil, -1)
		if err != nil {
			return err
		}
		code := resp.StatusCode
		resp.Body.Close()
		if code >= 200 && code < 300 {
			continue
		}
		if code == http.StatusMethodNotAllowed || code == http.StatusMovedPermanently {
			continue
		}
		return fmt.Errorf("webdav: MKCOL %s %d", prefix, code)
	}
	return nil
}

func (d *Driver) Put(ctx context.Context, key string, r io.Reader) error {
	// 记录 body 起始位置以便重试回卷;非 Seeker 无法安全重试(首次 PUT 可能已消费
	// body,直接复用会静默写空对象;codex 终审)。
	seeker, seekable := r.(io.Seeker)
	var startPos int64
	if seekable {
		p, serr := seeker.Seek(0, io.SeekCurrent)
		if serr != nil {
			return fmt.Errorf("webdav: 探测 body 位置: %w", serr)
		}
		startPos = p
	}

	resp, err := d.do(ctx, http.MethodPut, key, r, bodyLen(r))
	if err != nil {
		return err
	}

	// 缺父集合时 PUT 失败——各 WebDAV 服务器返回码不一:RFC 4918 是 409,但 Apache
	// mod_dav 实测返 403,亦见 404/500(真机验收发现)。故首次 PUT 非 2xx 且非 401
	// (认证失败,MKCOL 也修不了)即尝试 MKCOL 祖先后重试一次;仍失败则报错。
	if (resp.StatusCode < 200 || resp.StatusCode >= 300) && resp.StatusCode != http.StatusUnauthorized {
		resp.Body.Close()
		if !seekable {
			return fmt.Errorf("webdav: PUT %d 需重试建父目录,但 body 不可回卷", resp.StatusCode)
		}
		if err := d.mkcolParents(ctx, key); err != nil {
			return err
		}
		if _, err := seeker.Seek(startPos, io.SeekStart); err != nil {
			return fmt.Errorf("webdav: 重试 PUT 前 Seek: %w", err)
		}
		resp, err = d.do(ctx, http.MethodPut, key, r, bodyLen(r))
		if err != nil {
			return err
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return statusError("PUT", resp.StatusCode, snippet)
	}
	return nil
}

// statusError maps HTTP status to an operator-readable error (auth vs not found vs other).
func statusError(op string, code int, snippet []byte) error {
	msg := strings.TrimSpace(string(snippet))
	switch code {
	case http.StatusUnauthorized, http.StatusForbidden:
		if msg == "" {
			return fmt.Errorf("webdav: %s %d 认证或权限失败", op, code)
		}
		return fmt.Errorf("webdav: %s %d 认证或权限失败: %s", op, code, msg)
	case http.StatusNotFound:
		// OpenList 虚根不可写、挂载路径不存在时 PUT 也常返 404（并非「对象丢了」）
		if op == "PUT" {
			return fmt.Errorf("%w: 路径不存在或该 WebDAV 根不可写（OpenList 请用具体挂载路径，勿用 /dav 虚根）", storage.ErrNotFound)
		}
		return storage.ErrNotFound
	case http.StatusFound, http.StatusMovedPermanently, http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		return fmt.Errorf("webdav: %s %d 未跟随重定向（写操作不跟 3xx；若读失败请升级或检查代理）", op, code)
	case http.StatusMethodNotAllowed, http.StatusNotImplemented:
		return fmt.Errorf("webdav: %s %d 方法不被对端支持", op, code)
	default:
		if msg == "" {
			return fmt.Errorf("webdav: %s %d", op, code)
		}
		return fmt.Errorf("webdav: %s %d: %s", op, code, msg)
	}
}

// Open prefers HEAD+Range streaming (good TTFB). Falls back to full GET buffer when
// HEAD is unsupported or Content-Length is missing; Range-ignore servers fall back
// on first mid-file Read (see rangeReadSeekCloser).
func (d *Driver) Open(ctx context.Context, key string) (io.ReadSeekCloser, error) {
	size, ok, err := d.headSize(ctx, key)
	if err != nil {
		return nil, err
	}
	if !ok {
		return d.openBuffered(ctx, key)
	}
	return &rangeReadSeekCloser{d: d, ctx: ctx, key: key, size: size, offset: 0}, nil
}

// headSize returns (size, true, nil) when HEAD provides a usable Content-Length.
// (0, false, nil) means "caller should buffer"; non-nil err is hard failure.
func (d *Driver) headSize(ctx context.Context, key string) (int64, bool, error) {
	resp, err := d.do(ctx, http.MethodHead, key, nil, -1)
	if err != nil {
		return 0, false, err
	}
	resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return 0, false, storage.ErrNotFound
	case resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode == http.StatusNotImplemented:
		return 0, false, nil // buffer via GET
	// OpenList：HEAD 302 到直链，直链常拒 HEAD → 改走 GET 缓冲（可跟随 302）
	case resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusMovedPermanently ||
		resp.StatusCode == http.StatusTemporaryRedirect || resp.StatusCode == http.StatusPermanentRedirect:
		return 0, false, nil
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		return 0, false, statusError("HEAD", resp.StatusCode, nil)
	}
	cl := strings.TrimSpace(resp.Header.Get("Content-Length"))
	if cl == "" {
		return 0, false, nil
	}
	size, err := strconv.ParseInt(cl, 10, 64)
	if err != nil || size < 0 {
		return 0, false, nil
	}
	return size, true, nil
}

// openBuffered downloads the whole object (fallback path).
func (d *Driver) openBuffered(ctx context.Context, key string) (io.ReadSeekCloser, error) {
	resp, err := d.do(ctx, http.MethodGet, key, nil, -1)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, storage.ErrNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, statusError("GET", resp.StatusCode, snippet)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return &memRSC{r: bytes.NewReader(data)}, nil
}

func (d *Driver) Delete(ctx context.Context, key string) error {
	resp, err := d.do(ctx, http.MethodDelete, key, nil, -1)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode < 300 {
		return nil
	}
	snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	return statusError("DELETE", resp.StatusCode, snippet)
}

func (d *Driver) Exists(ctx context.Context, key string) (bool, error) {
	resp, err := d.do(ctx, http.MethodHead, key, nil, -1)
	if err != nil {
		return false, err
	}
	resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	// 302 等到直链：对象存在（内容在 Location）
	case http.StatusFound, http.StatusMovedPermanently, http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		return true, nil
	case http.StatusMethodNotAllowed, http.StatusNotImplemented:
		// Some servers disallow HEAD — treat as unknown, probe with GET size 0 range or Exists via Open.
		// Light probe: GET with Range bytes=0-0
		gr, gerr := d.getRange(ctx, key, 0)
		if gerr != nil {
			return false, gerr
		}
		defer gr.Body.Close()
		if gr.StatusCode == http.StatusNotFound {
			return false, nil
		}
		if gr.StatusCode == http.StatusOK || gr.StatusCode == http.StatusPartialContent {
			return true, nil
		}
		return false, statusError("GET", gr.StatusCode, nil)
	default:
		return false, statusError("HEAD", resp.StatusCode, nil)
	}
}

type memRSC struct {
	r *bytes.Reader
}

func (m *memRSC) Read(p []byte) (int, error)           { return m.r.Read(p) }
func (m *memRSC) Seek(off int64, wh int) (int64, error) { return m.r.Seek(off, wh) }
func (m *memRSC) Close() error                          { return nil }

type rangeReadSeekCloser struct {
	d      *Driver
	ctx    context.Context
	key    string
	size   int64
	offset int64
	body   io.ReadCloser
	// buf is set when the server ignores Range; subsequent IO uses memory.
	buf *bytes.Reader
}

func (r *rangeReadSeekCloser) Read(p []byte) (int, error) {
	if r.buf != nil {
		n, err := r.buf.Read(p)
		r.offset += int64(n)
		return n, err
	}
	if r.offset >= r.size {
		return 0, io.EOF
	}
	if r.body == nil {
		startOffset := r.offset
		resp, err := r.d.getRange(r.ctx, r.key, startOffset)
		if err != nil {
			return 0, err
		}
		// offset>0 却收 200：对端忽略 Range → 整对象缓冲后从 offset 继续（兼容烂 Dav）。
		if startOffset > 0 && resp.StatusCode == http.StatusOK {
			data, rerr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if rerr != nil {
				return 0, rerr
			}
			if int64(len(data)) > 0 {
				r.size = int64(len(data))
			}
			r.buf = bytes.NewReader(data)
			if _, err := r.buf.Seek(startOffset, io.SeekStart); err != nil {
				return 0, err
			}
			r.offset = startOffset
			n, err := r.buf.Read(p)
			r.offset += int64(n)
			return n, err
		}
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
			snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
			resp.Body.Close()
			return 0, statusError("GET", resp.StatusCode, snippet)
		}
		r.body = resp.Body
	}
	n, err := r.body.Read(p)
	r.offset += int64(n)
	if err == io.EOF {
		r.body.Close()
		r.body = nil
		if r.offset < r.size {
			err = nil // 段读完但对象未尽,下次重开
		}
	}
	return n, err
}

func (r *rangeReadSeekCloser) Seek(off int64, whence int) (int64, error) {
	var abs int64
	switch whence {
	case io.SeekStart:
		abs = off
	case io.SeekCurrent:
		abs = r.offset + off
	case io.SeekEnd:
		abs = r.size + off
	default:
		return 0, fmt.Errorf("webdav: 无效 whence %d", whence)
	}
	if abs < 0 {
		return 0, fmt.Errorf("webdav: 负偏移")
	}
	if r.buf != nil {
		n, err := r.buf.Seek(abs, io.SeekStart)
		r.offset = n
		return n, err
	}
	if abs != r.offset && r.body != nil {
		r.body.Close()
		r.body = nil
	}
	r.offset = abs
	return abs, nil
}

func (r *rangeReadSeekCloser) Close() error {
	if r.body != nil {
		err := r.body.Close()
		r.body = nil
		return err
	}
	return nil
}
