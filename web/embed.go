// Package web 嵌入前端构建产物（web/dist）。未执行 make web 时 dist 仅含
// .gitkeep，server 对缺失 index.html 回退内置提示页。
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// DistFS 返回以 dist 为根的文件系统。
func DistFS() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}
