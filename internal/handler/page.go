package handler

import (
	"net/http"
	"strconv"
)

// ParsePage 从 query 解析 page/limit，缺省与越界按 def/max 钳制。
// page 最小 1；limit 在 (0, max] 内，非法或缺失用 defLimit。
func ParsePage(r *http.Request, defPage, defLimit, maxLimit int) (page, limit int) {
	page, limit = defPage, defLimit
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 50
	}
	if maxLimit <= 0 {
		maxLimit = limit
	}
	if v := r.URL.Query().Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			page = n
		}
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	return page, limit
}

// ParseDaysLimit 解析 days/limit 类 query（管理端 referer 等）；非法则用默认值。
func ParseDaysLimit(r *http.Request, defDays, defLimit, maxLimit int) (days, limit int) {
	days, limit = defDays, defLimit
	if v := r.URL.Query().Get("days"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			days = n
		}
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if maxLimit > 0 && limit > maxLimit {
		limit = maxLimit
	}
	return days, limit
}
