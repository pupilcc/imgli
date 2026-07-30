package handler

import (
	"github.com/yixian-huang/imgli/internal/linkbuilder"
	"github.com/yixian-huang/imgli/internal/model"
)

// imageLinks 构造直链/markdown 等；缩略图 URL 始终用稳定 key（存储与缓存键）。
// ref 优先 slug，否则 key——与列表/上传/管理端 DTO 口径一致。
func imageLinks(base, key, ext, name string, slug *string) linkbuilder.Links {
	ref := key
	if slug != nil && *slug != "" {
		ref = *slug
	}
	links := linkbuilder.Build(base, ref, ext, name)
	links.ThumbnailURL = base + "/t/" + key + ".jpg"
	return links
}

// imageLinksFrom 从 Image 模型取 key/slug/ext/name。
func imageLinksFrom(base string, img *model.Image) linkbuilder.Links {
	return imageLinks(base, img.Key, img.Ext, img.Name, img.Slug)
}
