package handler

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/yixian-huang/imgli/internal/model"
)

const SessionCookie = "imgli_session"

// Principal 是通过认证的请求主体。Scope："full"（session 或 full token）| "upload"。
type Principal struct {
	User  *model.User
	Scope string
}

type principalKey struct{}

func PrincipalFrom(r *http.Request) *Principal {
	p, _ := r.Context().Value(principalKey{}).(*Principal)
	return p
}

// UserResolver 由 server 装配（session 查 auth service，Bearer 查 apitoken service）。
type UserResolver interface {
	UserBySession(token string) (*model.User, error)
	UserByAPIToken(token string) (*model.User, string, error)
}

// Auth 解析身份注入 context。Bearer 优先；解析失败按匿名放行，由 RequireUser 拦截。
func Auth(res UserResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var p *Principal
			if ah := r.Header.Get("Authorization"); strings.HasPrefix(ah, "Bearer ") {
				if u, scope, err := res.UserByAPIToken(strings.TrimPrefix(ah, "Bearer ")); err == nil && u != nil {
					p = &Principal{User: u, Scope: scope}
				}
			} else if c, err := r.Cookie(SessionCookie); err == nil {
				if u, err := res.UserBySession(c.Value); err == nil && u != nil {
					p = &Principal{User: u, Scope: "full"}
				}
			}
			if p != nil {
				r = r.WithContext(context.WithValue(r.Context(), principalKey{}, p))
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if PrincipalFrom(r) == nil {
			Fail(w, http.StatusUnauthorized, CodeUnauthorized, "请先登录")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireUserOrAnon 放行已登录请求与"真正无凭证"的匿名请求；拦截"出示了凭证但
// 解析不出身份"的请求——过期/被吊销的 session cookie、拼错或已吊销的 Bearer
// token。这类请求若被 Auth 悄悄当无凭证放行，会在游客上传路径里变成不明确的
// user_id=NULL 匿名上传（开关开时静默丢失归属）或困惑的 403（开关关时）。
// 只有连凭证都没带的请求才应落到匿名/游客分支。
func RequireUserOrAnon(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if PrincipalFrom(r) != nil {
			next.ServeHTTP(w, r)
			return
		}
		presented := strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !presented {
			if _, err := r.Cookie(SessionCookie); err == nil {
				presented = true
			}
		}
		if presented {
			Fail(w, http.StatusUnauthorized, CodeUnauthorized, "登录状态已失效，请重新登录")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireFullScope 拦截仅上传权限的 API Token（上传端点在计划②b 单独放行）。
func RequireFullScope(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p := PrincipalFrom(r); p != nil && p.Scope != "full" {
			Fail(w, http.StatusForbidden, CodeForbidden, "Token 权限不足")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// OriginCheck 对携带 Origin 的写方法做同源校验（配合 SameSite=Lax 的 CSRF 防线）。
// 放行：读方法、Bearer 请求（无浏览器凭据，无 CSRF 风险）、Origin 缺失、
// Origin 宿主 == 请求 Host（直连/反代/开发代理均成立）、Origin == BaseURL。
func OriginCheck(baseURL string) func(http.Handler) http.Handler {
	allowed := ""
	if u, err := url.Parse(baseURL); err == nil {
		allowed = u.Scheme + "://" + u.Host
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				next.ServeHTTP(w, r)
				return
			}
			if strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
				next.ServeHTTP(w, r)
				return
			}
			if o := r.Header.Get("Origin"); o != "" {
				scheme := "http"
				if r.TLS != nil {
					scheme = "https"
				}
				sameOrigin := o == scheme+"://"+r.Host
				if !sameOrigin && o != allowed {
					Fail(w, http.StatusForbidden, CodeForbidden, "跨站请求被拒绝")
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
