package handler

import "net/http"

// RequireAdmin 管理员门禁（挂在 RequireUser+RequireFullScope 之后，Principal 恒非空）。
// 裁决 1：403 不装 404——admin 面板无防枚举需求。
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p := PrincipalFrom(r); p == nil || p.User == nil || !p.User.IsAdmin {
			Fail(w, http.StatusForbidden, CodeForbidden, "需要管理员权限")
			return
		}
		next.ServeHTTP(w, r)
	})
}
