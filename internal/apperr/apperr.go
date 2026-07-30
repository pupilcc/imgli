// Package apperr 提供可映射为 4xx 的客户端错误标记，避免 handler 维护 sentinel 清单。
package apperr

import "errors"

// Client 是可安全展示给调用方的校验/业务错误（HTTP 4xx）。
type Client interface {
	error
	ClientError()
}

type client struct{ msg string }

func (e *client) Error() string { return e.msg }
func (e *client) ClientError()   {}

// New 构造客户端错误；请用包级 var 保存返回值以便 errors.Is 指针相等匹配。
func New(msg string) error { return &client{msg: msg} }

// IsClient 判断 err（含 wrap）是否为客户端错误。
func IsClient(err error) bool {
	var c Client
	return errors.As(err, &c)
}
