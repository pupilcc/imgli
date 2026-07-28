package webdav

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/yixian-huang/imgli/internal/storage"
)

// mockWebDAV 内存 WebDAV 桩：按 URL path 存对象，dirs 记 MKCOL 集合。
type mockWebDAV struct {
	mu sync.Mutex

	objects map[string][]byte
	dirs    map[string]bool

	lastMethod    string
	lastPath      string
	lastAuth      string
	lastContentLn int64
	lastMkcols    []string

	requireAuth bool
	username    string
	password    string

	// forceStatus: 若非 0，对该 method 强制返回该状态码
	forceStatus map[string]int
	// ignoreRange: GET 无视 Range 恒返 200 全量
	ignoreRange bool
	// missingParentCode: 缺父集合 PUT 的返回码(0=默认 409;真 Apache mod_dav 返 403)
	missingParentCode int
}

func newMockWebDAV() *mockWebDAV {
	return &mockWebDAV{
		objects:     make(map[string][]byte),
		dirs:        make(map[string]bool),
		forceStatus: make(map[string]int),
	}
}

func (m *mockWebDAV) authOK(r *http.Request) bool {
	if !m.requireAuth {
		return true
	}
	user, pass, ok := r.BasicAuth()
	return ok && user == m.username && pass == m.password
}

// parentDir 返回 path 的父集合路径(去尾文件名);根下文件返回 ""。
func parentDir(path string) string {
	path = strings.TrimSuffix(path, "/")
	i := strings.LastIndex(path, "/")
	if i <= 0 {
		return ""
	}
	return path[:i]
}

func (m *mockWebDAV) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.lastMethod = r.Method
	m.lastPath = r.URL.Path
	m.lastAuth = r.Header.Get("Authorization")
	m.lastContentLn = r.ContentLength

	if m.requireAuth && !m.authOK(r) {
		w.Header().Set("WWW-Authenticate", `Basic realm="webdav"`)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if code, ok := m.forceStatus[r.Method]; ok && code != 0 {
		w.WriteHeader(code)
		return
	}

	storeKey := r.URL.Path

	switch r.Method {
	case http.MethodPut:
		parent := parentDir(storeKey)
		// 父集合不存在(且非根) → 缺父码(默认 409;真 Apache mod_dav 返 403)
		if parent != "" && !m.dirs[parent] {
			code := m.missingParentCode
			if code == 0 {
				code = http.StatusConflict
			}
			w.WriteHeader(code)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		m.objects[storeKey] = body
		w.WriteHeader(http.StatusCreated)

	case http.MethodGet:
		data, ok := m.objects[storeKey]
		if !ok {
			http.NotFound(w, r)
			return
		}
		rangeHdr := r.Header.Get("Range")
		if strings.HasPrefix(rangeHdr, "bytes=") && !m.ignoreRange {
			spec := strings.TrimPrefix(rangeHdr, "bytes=")
			parts := strings.SplitN(spec, "-", 2)
			start, _ := strconv.ParseInt(parts[0], 10, 64)
			end := int64(len(data) - 1)
			if len(parts) > 1 && parts[1] != "" {
				end, _ = strconv.ParseInt(parts[1], 10, 64)
			}
			if start < 0 || start >= int64(len(data)) {
				w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
				return
			}
			if end >= int64(len(data)) {
				end = int64(len(data) - 1)
			}
			chunk := data[start : end+1]
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
			w.Header().Set("Content-Length", strconv.Itoa(len(chunk)))
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(chunk)
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)

	case http.MethodHead:
		data, ok := m.objects[storeKey]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.WriteHeader(http.StatusOK)

	case http.MethodDelete:
		if _, ok := m.objects[storeKey]; !ok {
			http.NotFound(w, r)
			return
		}
		delete(m.objects, storeKey)
		w.WriteHeader(http.StatusNoContent)

	case "MKCOL":
		// 规范化: 去尾 /
		col := strings.TrimSuffix(storeKey, "/")
		m.lastMkcols = append(m.lastMkcols, col)
		if m.dirs[col] {
			w.WriteHeader(http.StatusMethodNotAllowed) // 已存在
			return
		}
		m.dirs[col] = true
		w.WriteHeader(http.StatusCreated)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func testDriver(t *testing.T, mock *mockWebDAV, cfg map[string]string) *Driver {
	t.Helper()
	srv := httptest.NewServer(mock)
	t.Cleanup(srv.Close)

	if cfg == nil {
		cfg = map[string]string{}
	}
	// endpoint 指向桩;若调用方未设则用 srv.URL
	if strings.TrimSpace(cfg["endpoint"]) == "" {
		cfg["endpoint"] = srv.URL
	} else if !strings.HasPrefix(cfg["endpoint"], "http") {
		// 允许相对 base path 拼到桩 URL
		cfg["endpoint"] = srv.URL + cfg["endpoint"]
	} else {
		// 已是完整 URL 时(含 scheme)直接用桩 URL 覆盖 host,保留 path 部分较复杂;
		// 测试统一:若 endpoint 已是完整 http URL 且非桩,替换为桩 URL(+可选 path)。
		// 实际测试均用空 endpoint 或 srv 拼装,见各用例。
		cfg["endpoint"] = srv.URL
	}

	d, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// 直连桩(Client 默认即可,endpoint 已是 srv.URL)
	d.Client = srv.Client()
	return d
}

func TestWebDAVPutOpenDelete(t *testing.T) {
	mock := newMockWebDAV()
	// 预建父目录,隔离 MKCOL 行为
	mock.dirs["/a"] = true
	d := testDriver(t, mock, map[string]string{
		"username": "u",
		"password": "p",
	})
	ctx := context.Background()

	payload := []byte("hello-webdav-png")
	if err := d.Put(ctx, "a/b.png", bytes.NewReader(payload)); err != nil {
		t.Fatalf("Put: %v", err)
	}

	mock.mu.Lock()
	path := mock.lastPath
	mock.mu.Unlock()
	if path != "/a/b.png" {
		t.Errorf("PUT path = %q, want /a/b.png", path)
	}

	rsc, err := d.Open(ctx, "a/b.png")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	got, err := io.ReadAll(rsc)
	rsc.Close()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("Open body = %q, want %q", got, payload)
	}

	if err := d.Delete(ctx, "a/b.png"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err = d.Open(ctx, "a/b.png")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("Open after Delete: err=%v, want ErrNotFound", err)
	}
}

// 真机验收发现:Apache mod_dav 对缺父目录 PUT 返 403(非 RFC 的 409)。驱动须对
// 首次 PUT 非 2xx(非 401)即 MKCOL 祖先重试,不能只认 409。
func TestWebDAVMkcolOn403MissingParent(t *testing.T) {
	mock := newMockWebDAV()
	mock.missingParentCode = http.StatusForbidden // 模拟真 Apache
	d := testDriver(t, mock, nil)
	ctx := context.Background()
	if err := d.Put(ctx, "p/q/r.png", bytes.NewReader([]byte("mod-dav-403"))); err != nil {
		t.Fatalf("Put(403 缺父)应经 MKCOL 重试成功: %v", err)
	}
	mock.mu.Lock()
	_, ok := mock.objects["/p/q/r.png"]
	ncol := len(mock.lastMkcols)
	mock.mu.Unlock()
	if !ok || ncol < 2 {
		t.Errorf("403 缺父后应 MKCOL(%d)+存对象(%v)", ncol, ok)
	}
}

func TestWebDAVMkcolOn409(t *testing.T) {
	mock := newMockWebDAV()
	// 不预建 dirs:PUT x/y/z.png 应 409 → MKCOL x、x/y → 重试 PUT
	d := testDriver(t, mock, nil)
	ctx := context.Background()

	payload := []byte("nested-payload")
	if err := d.Put(ctx, "x/y/z.png", bytes.NewReader(payload)); err != nil {
		t.Fatalf("Put: %v", err)
	}

	mock.mu.Lock()
	mkcols := append([]string(nil), mock.lastMkcols...)
	_, ok := mock.objects["/x/y/z.png"]
	mock.mu.Unlock()

	wantCols := []string{"/x", "/x/y"}
	if len(mkcols) < 2 {
		t.Fatalf("MKCOL count=%d paths=%v, want at least %v", len(mkcols), mkcols, wantCols)
	}
	// 应包含 /x 与 /x/y(顺序:先浅后深)
	if mkcols[0] != "/x" || mkcols[1] != "/x/y" {
		t.Errorf("MKCOL order = %v, want %v first", mkcols, wantCols)
	}
	if !ok {
		t.Error("object /x/y/z.png not stored after MKCOL retry")
	}

	rsc, err := d.Open(ctx, "x/y/z.png")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	got, err := io.ReadAll(rsc)
	rsc.Close()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("Open body = %q, want %q", got, payload)
	}
}

func TestWebDAVRangeSeek(t *testing.T) {
	mock := newMockWebDAV()
	d := testDriver(t, mock, nil)
	ctx := context.Background()

	data := make([]byte, 100)
	for i := range data {
		data[i] = byte(i)
	}
	if err := d.Put(ctx, "range.bin", bytes.NewReader(data)); err != nil {
		t.Fatalf("Put: %v", err)
	}

	rsc, err := d.Open(ctx, "range.bin")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer rsc.Close()

	end, err := rsc.Seek(0, io.SeekEnd)
	if err != nil || end != 100 {
		t.Fatalf("SeekEnd = %d, %v; want 100", end, err)
	}
	if _, err := rsc.Seek(10, io.SeekStart); err != nil {
		t.Fatalf("SeekStart(10): %v", err)
	}
	got, err := io.ReadAll(rsc)
	if err != nil {
		t.Fatalf("ReadAll from 10: %v", err)
	}
	if len(got) != 90 {
		t.Fatalf("len(got)=%d, want 90", len(got))
	}
	if !bytes.Equal(got, data[10:]) {
		t.Errorf("range content mismatch")
	}

	// forceGet200: offset>0 返 200 → Read 报错
	mock2 := newMockWebDAV()
	mock2.ignoreRange = true
	d2 := testDriver(t, mock2, nil)
	if err := d2.Put(ctx, "r.bin", bytes.NewReader(data)); err != nil {
		t.Fatalf("Put2: %v", err)
	}
	rsc2, err := d2.Open(ctx, "r.bin")
	if err != nil {
		t.Fatalf("Open2: %v", err)
	}
	defer rsc2.Close()
	if _, err := rsc2.Seek(10, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 10)
	if _, err := rsc2.Read(buf); err == nil {
		t.Error("offset>0 收 200 应报错(防读错字节),got nil")
	}
}

func TestWebDAVBasicAuth(t *testing.T) {
	ctx := context.Background()

	// username/password 设 → Basic
	mock := newMockWebDAV()
	mock.dirs["/a"] = true
	d := testDriver(t, mock, map[string]string{
		"username": "alice",
		"password": "s3cret",
	})
	if err := d.Put(ctx, "a/f.png", bytes.NewReader([]byte("x"))); err != nil {
		t.Fatalf("Put: %v", err)
	}
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("alice:s3cret"))
	mock.mu.Lock()
	auth := mock.lastAuth
	mock.mu.Unlock()
	if auth != want {
		t.Errorf("Authorization=%q, want %q", auth, want)
	}

	// username 空 → 无 Authorization
	mock2 := newMockWebDAV()
	mock2.dirs["/a"] = true
	d2 := testDriver(t, mock2, map[string]string{
		"username": "",
		"password": "ignored",
	})
	if err := d2.Put(ctx, "a/f.png", bytes.NewReader([]byte("y"))); err != nil {
		t.Fatalf("Put no-auth: %v", err)
	}
	mock2.mu.Lock()
	auth2 := mock2.lastAuth
	mock2.mu.Unlock()
	if auth2 != "" {
		t.Errorf("username 空时 Authorization 应为空, got %q", auth2)
	}
}

func TestWebDAVExists(t *testing.T) {
	mock := newMockWebDAV()
	d := testDriver(t, mock, nil)
	ctx := context.Background()

	ok, err := d.Exists(ctx, "missing")
	if err != nil || ok {
		t.Errorf("Exists missing: ok=%v err=%v; want false,nil", ok, err)
	}

	if err := d.Put(ctx, "hit", bytes.NewReader([]byte("y"))); err != nil {
		t.Fatalf("Put: %v", err)
	}
	ok, err = d.Exists(ctx, "hit")
	if err != nil || !ok {
		t.Errorf("Exists hit: ok=%v err=%v; want true,nil", ok, err)
	}

	// HEAD 403 → error(forceStatus 作用于 HEAD)
	mock.forceStatus["HEAD"] = 403
	_, err = d.Exists(ctx, "hit")
	if err == nil {
		t.Error("Exists with 403: want error")
	}
}

func TestWebDAVOsFileContentLength(t *testing.T) {
	mock := newMockWebDAV()
	d := testDriver(t, mock, nil)
	f, err := os.CreateTemp(t.TempDir(), "up-*")
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("file-body-1234567890")
	if _, err := f.Write(payload); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	if err := d.Put(context.Background(), "f.png", f); err != nil {
		t.Fatalf("Put os.File: %v", err)
	}
	mock.mu.Lock()
	cl := mock.lastContentLn
	mock.mu.Unlock()
	if cl != int64(len(payload)) {
		t.Errorf("Content-Length=%d, want %d(不应为 -1/chunked)", cl, len(payload))
	}
}

func TestNewMissingEndpoint(t *testing.T) {
	_, err := New(map[string]string{})
	if err == nil {
		t.Fatal("want error for empty cfg")
	}
	if !strings.Contains(err.Error(), "缺少必填配置 endpoint") {
		t.Errorf("err=%v", err)
	}
}

// codex 终审 F2:首次 PUT 失败但 body 非 Seeker → 不得静默重试写空对象,应报错。
type nonSeekerReader struct{ r io.Reader }

func (n nonSeekerReader) Read(p []byte) (int, error) { return n.r.Read(p) }

func TestWebDAVPutNonSeekerNoSilentRetry(t *testing.T) {
	mock := newMockWebDAV()
	mock.missingParentCode = http.StatusForbidden // 触发重试路径
	d := testDriver(t, mock, nil)
	body := nonSeekerReader{r: bytes.NewReader([]byte("x/y deep body"))}
	if err := d.Put(context.Background(), "deep/dir/f.png", body); err == nil {
		t.Error("非 Seeker body 且需重试应报错(防写空对象),got nil")
	}
}

// codex 终审 F3:endpoint 内联 userinfo 应拒(防明文凭据绕过打码回显)。
func TestWebDAVRejectUserinfo(t *testing.T) {
	_, err := New(map[string]string{"endpoint": "https://user:pass@dav.example.com/imgli"})
	if err == nil {
		t.Error("endpoint 含 userinfo 应报错")
	}
}

func TestNewInvalidEndpoint(t *testing.T) {
	cases := []string{
		"not-a-url",
		"ftp://example.com",
		"http://",
		"://bad",
	}
	for _, ep := range cases {
		_, err := New(map[string]string{"endpoint": ep})
		if err == nil {
			t.Errorf("endpoint %q: want error", ep)
			continue
		}
		if !strings.Contains(err.Error(), "endpoint 非法 URL") {
			t.Errorf("endpoint %q: err=%v", ep, err)
		}
	}
}
