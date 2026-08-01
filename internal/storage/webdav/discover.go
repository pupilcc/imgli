package webdav

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"
)

// WritableHint 是「测试连接」失败时给出的可写挂载建议（无密码）。
type WritableHint struct {
	Name     string `json:"name"`     // 路径段（已解码）
	Endpoint string `json:"endpoint"` // 完整 WebDAV 基址
}

var (
	hrefRe       = regexp.MustCompile(`(?i)<[^:>]*:?href[^>]*>([^<]+)</`)
	propfindBody = []byte(`<?xml version="1.0" encoding="utf-8"?><d:propfind xmlns:d="DAV:"><d:prop><d:resourcetype/></d:prop></d:propfind>`)
	maxMountProbe = 8
)

// ListChildCollectionNames PROPFIND Depth:1，返回相对当前 baseURL 的一级子路径名（已 URL 解码）。
// 用于 OpenList 等「虚根下列出挂载」的场景。
func (d *Driver) ListChildCollectionNames(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "PROPFIND", d.baseURL+"/", bytes.NewReader(propfindBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Depth", "1")
	req.Header.Set("Content-Type", "application/xml; charset=utf-8")
	if d.username != "" {
		req.SetBasicAuth(d.username, d.password)
	}
	resp, err := d.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusMultiStatus && (resp.StatusCode < 200 || resp.StatusCode >= 300) {
		return nil, statusError("PROPFIND", resp.StatusCode, body)
	}
	return childNamesFromPropfind(d.baseURL, string(body)), nil
}

// SuggestWritableMounts 枚举一级子路径并做写/删探针，返回可写挂载建议。
func (d *Driver) SuggestWritableMounts(ctx context.Context, max int) ([]WritableHint, error) {
	if max <= 0 || max > maxMountProbe {
		max = maxMountProbe
	}
	names, err := d.ListChildCollectionNames(ctx)
	if err != nil {
		return nil, err
	}
	var out []WritableHint
	for _, name := range names {
		if len(out) >= max {
			break
		}
		if name == "" || name == "." || name == ".." {
			continue
		}
		childEP := joinEndpoint(d.baseURL, name)
		child, err := New(map[string]string{
			"endpoint": childEP,
			"username": d.username,
			"password": d.password,
		})
		if err != nil {
			continue
		}
		if d.Client != nil {
			child.Client = d.Client
		}
		if err := probeWritable(ctx, child); err != nil {
			continue
		}
		out = append(out, WritableHint{Name: name, Endpoint: childEP})
	}
	return out, nil
}

// FormatWritableHints 运营可见的一句话建议。
func FormatWritableHints(hints []WritableHint) string {
	if len(hints) == 0 {
		return ""
	}
	parts := make([]string, 0, len(hints))
	for _, h := range hints {
		if h.Name != "" {
			parts = append(parts, h.Name+" → "+h.Endpoint)
		} else {
			parts = append(parts, h.Endpoint)
		}
	}
	return "探测到可写挂载，请将 endpoint 改为其一：" + strings.Join(parts, "；")
}

// HintVirtualRoot OpenList 类虚根的固定补充说明（无探测结果时用）。
func HintVirtualRoot() string {
	return "若使用 OpenList：endpoint 须为具体挂载路径（如 https://host/dav/挂载名），不要用 /dav 虚根；也不要填尚未在 OpenList 中创建的子路径"
}

// DiscoverWritableMounts 从配置建议可写挂载。
func DiscoverWritableMounts(ctx context.Context, cfg map[string]string, max int) ([]WritableHint, error) {
	d, err := New(cfg)
	if err != nil {
		return nil, err
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
	}
	return d.SuggestWritableMounts(ctx, max)
}

func probeWritable(ctx context.Context, d *Driver) error {
	key := ".imgli-mount-probe-" + randHex(4)
	if err := d.Put(ctx, key, bytes.NewReader([]byte("ok"))); err != nil {
		return err
	}
	_ = d.Delete(ctx, key)
	return nil
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func joinEndpoint(base, name string) string {
	// 展示与配置用未转义路径段；HTTP 请求由 net/url / encodePath 处理编码
	return strings.TrimRight(base, "/") + "/" + strings.Trim(name, "/")
}

func childNamesFromPropfind(baseURL, xmlBody string) []string {
	basePath := pathOfURL(baseURL)
	return uniqueChildNames(basePath, hrefRe.FindAllStringSubmatch(xmlBody, -1))
}

func uniqueChildNames(basePath string, ms [][]string) []string {
	seen := map[string]struct{}{}
	var names []string
	for _, m := range ms {
		if len(m) < 2 {
			continue
		}
		name := relativeChildName(basePath, m[1])
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	return names
}

func pathOfURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		p := strings.TrimRight(raw, "/")
		if p == "" {
			return "/"
		}
		return p
	}
	p := u.EscapedPath()
	if p == "" {
		p = "/"
	}
	// 用 Unescape 再 Clean，便于和解码后的 href 比较
	if dec, err := url.PathUnescape(p); err == nil {
		p = dec
	}
	p = path.Clean(p)
	if p != "/" {
		p = strings.TrimSuffix(p, "/")
	}
	return p
}

func relativeChildName(basePath, href string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	if strings.Contains(href, "://") {
		u, err := url.Parse(href)
		if err != nil {
			return ""
		}
		href = u.Path
		if href == "" {
			href = "/"
		}
	}
	if dec, err := url.PathUnescape(href); err == nil {
		href = dec
	}
	href = path.Clean("/" + strings.Trim(href, "/"))
	if href != "/" {
		href = strings.TrimSuffix(href, "/")
	}
	basePath = strings.TrimSuffix(basePath, "/")
	if basePath == "" {
		basePath = "/"
	}
	if href == basePath {
		return ""
	}
	var rest string
	if basePath == "/" {
		rest = strings.TrimPrefix(href, "/")
	} else if strings.HasPrefix(href, basePath+"/") {
		rest = href[len(basePath)+1:]
	} else {
		return ""
	}
	rest = strings.Trim(rest, "/")
	if rest == "" {
		return ""
	}
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		rest = rest[:i]
	}
	return rest
}
