// Package linkbuilder 组装图片的全格式外链（lsky 兼容集合）。
package linkbuilder

import "html"

type Links struct {
	URL          string `json:"url"`
	Markdown     string `json:"markdown"`
	HTML         string `json:"html"`
	BBCode       string `json:"bbcode"`
	ThumbnailURL string `json:"thumbnail_url"`
	ShareURL     string `json:"share_url,omitempty"`
}

// Build 由链接基址、直链 key、扩展名与显示名组装全格式链接。
// linkBase 不含尾斜杠。name 在 HTML alt 中做 HTML 转义防注入。
// key 参数在 vanity slug 场景下可为 slug（用于 /i 与 /s）；缩略图路径由调用方按稳定 key 覆写。
func Build(linkBase, key, ext, name string) Links {
	u := linkBase + "/i/" + key + "." + ext
	return Links{
		URL:          u,
		Markdown:     "![" + name + "](" + u + ")",
		HTML:         `<img src="` + u + `" alt="` + html.EscapeString(name) + `">`,
		BBCode:       "[img]" + u + "[/img]",
		ThumbnailURL: linkBase + "/t/" + key + ".jpg",
		ShareURL:     linkBase + "/s/" + key,
	}
}
