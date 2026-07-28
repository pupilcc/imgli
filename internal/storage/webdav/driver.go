// Package webdav 实现 WebDAV 存储驱动(HTTP Basic 认证,零额外依赖)。
package webdav

import (
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
		Timeout: 30 * time.Second,
		// 禁自动重定向:WebDAV 的 3xx 不应被静默改成 GET 致误报写/删成功,也防
		// https→http 降级携带 Basic 凭据(codex 终审)。ErrUseLastResponse 让驱动
		// 看到原始 3xx 自行判定。
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
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
		return fmt.Errorf("webdav: PUT %d: %s", resp.StatusCode, snippet)
	}
	return nil
}

func (d *Driver) Open(ctx context.Context, key string) (io.ReadSeekCloser, error) {
	resp, err := d.do(ctx, http.MethodHead, key, nil, -1)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, storage.ErrNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("webdav: HEAD %d", resp.StatusCode)
	}
	size, err := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("webdav: Content-Length: %w", err)
	}
	return &rangeReadSeekCloser{d: d, ctx: ctx, key: key, size: size, offset: 0}, nil
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
	return fmt.Errorf("webdav: DELETE %d: %s", resp.StatusCode, snippet)
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
	default:
		return false, fmt.Errorf("webdav: HEAD %d", resp.StatusCode)
	}
}

type rangeReadSeekCloser struct {
	d      *Driver
	ctx    context.Context
	key    string
	size   int64
	offset int64
	body   io.ReadCloser
}

func (r *rangeReadSeekCloser) Read(p []byte) (int, error) {
	if r.offset >= r.size {
		return 0, io.EOF
	}
	if r.body == nil {
		startOffset := r.offset
		resp, err := r.d.getRange(r.ctx, r.key, startOffset)
		if err != nil {
			return 0, err
		}
		// offset>0 却收 200(服务端忽略 Range)会从第 0 字节返回——必须拒
		if startOffset > 0 && resp.StatusCode == 200 {
			resp.Body.Close()
			return 0, fmt.Errorf("webdav: 服务端忽略 Range(返回 200),offset=%d", startOffset)
		}
		if resp.StatusCode != 200 && resp.StatusCode != 206 {
			resp.Body.Close()
			return 0, fmt.Errorf("webdav: GET range %d", resp.StatusCode)
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
	if abs != r.offset && r.body != nil {
		r.body.Close()
		r.body = nil
	}
	r.offset = abs
	return abs, nil
}

func (r *rangeReadSeekCloser) Close() error {
	if r.body != nil {
		return r.body.Close()
	}
	return nil
}
