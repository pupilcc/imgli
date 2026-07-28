package handler

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
)

type ctxKey int

const ctxKeyIP ctxKey = iota

func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic recovered",
					"err", rec,
					"method", r.Method,
					"path", r.URL.Path,
					"stack", string(debug.Stack()))
				Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// RealIP 解析客户端 IP 存入 context。trustProxy 为 true 时取
// X-Forwarded-For 的最右值——那是可信反代追加的；最左值可被客户端伪造。
// 仅在确有一层可信反代（img.li 形态）时开启，见 spec §7。
func RealIP(trustProxy bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := remoteIP(r)
			if trustProxy {
				if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
					parts := strings.Split(xff, ",")
					cand := strings.TrimSpace(parts[len(parts)-1])
					if net.ParseIP(cand) != nil {
						ip = cand
					}
				}
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKeyIP, ip)))
		})
	}
}

func ClientIP(r *http.Request) string {
	if v, ok := r.Context().Value(ctxKeyIP).(string); ok {
		return v
	}
	return remoteIP(r)
}

func remoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
