package server

import (
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"

	"github.com/yixian-huang/imgli/internal/handler"
)

func init() {
	// Go 默认不识别 .webmanifest,直出会落 octet-stream 触发浏览器告警;显式注册。
	_ = mime.AddExtensionType(".webmanifest", "application/manifest+json")
}

// noBuildHTML：dist 未构建（无 index.html）时的提示页。
const noBuildHTML = `<!doctype html><meta charset="utf-8"><title>img.li</title>
<p style="font-family:system-ui;margin:40px">前端未构建：请运行 <code>make web</code> 后重启；开发请在 web/ 下 <code>npm run dev</code>（代理到 :8686）。</p>`

// mountWeb 挂载 SPA：/assets/* 走 immutable 静态服务；其余 GET/HEAD 未匹配
// 且不属 API/直链/带扩展名的路径回落 index.html。API 路径 404 信封不变
//（/api/v1 子路由在本方法执行前已捕获 JSON NotFound）。
func (s *Server) mountWeb(dist fs.FS) {
	fileServer := http.FileServer(http.FS(dist))
	s.mux.Handle("/assets/*", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if strings.HasSuffix(r.URL.Path, "/") || !fs.ValidPath(name) {
			handler.Fail(w, http.StatusNotFound, handler.CodeNotFound, "资源不存在")
			return
		}
		if st, err := fs.Stat(dist, name); err != nil || st.IsDir() {
			handler.Fail(w, http.StatusNotFound, handler.CodeNotFound, "资源不存在")
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		fileServer.ServeHTTP(w, r)
	}))

	index := func(w http.ResponseWriter, r *http.Request) {
		b, err := fs.ReadFile(dist, "index.html")
		if err != nil {
			b = []byte(noBuildHTML)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(b)
	}
	s.mux.Get("/", index)
	s.mux.Head("/", index)

	jsonNotFound := func(w http.ResponseWriter, r *http.Request) {
		handler.Fail(w, http.StatusNotFound, handler.CodeNotFound, "资源不存在")
	}
	s.mux.NotFound(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if (r.Method != http.MethodGet && r.Method != http.MethodHead) ||
			strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/i/") ||
			strings.HasPrefix(p, "/t/") || strings.HasPrefix(p, "/assets/") {
			jsonNotFound(w, r)
			return
		}
		// dist 根文件（favicon 等）直出；目录路径（如无尾斜杠的 /assets）不暴露清单；
		// 其余带扩展名的未命中不回落 SPA
		name := strings.TrimPrefix(path.Clean(p), "/")
		if name != "" && fs.ValidPath(name) {
			if st, err := fs.Stat(dist, name); err == nil {
				if st.IsDir() {
					jsonNotFound(w, r)
					return
				}
				http.ServeFileFS(w, r, dist, name)
				return
			}
		}
		if path.Ext(name) != "" {
			jsonNotFound(w, r)
			return
		}
		index(w, r)
	})
}
