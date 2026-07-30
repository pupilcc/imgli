package handler

import (
	"strings"
	"time"

	"github.com/yixian-huang/imgli/internal/linkbuilder"
	"github.com/yixian-huang/imgli/internal/service/imagesvc"
)

// ImageItemDTO 用户侧列表/详情公共字段（JSON 契约与历史 map DTO 一致）。
type ImageItemDTO struct {
	Key               string           `json:"key"`
	Slug              any              `json:"slug"` // string | null
	Name              string           `json:"name"`
	Ext               string           `json:"ext"`
	Size              int64            `json:"size"`
	Width             int              `json:"width"`
	Height            int              `json:"height"`
	Visibility        string           `json:"visibility"`
	AlbumID           *uint64          `json:"album_id"`
	CreatedAt         string           `json:"created_at"`
	ExpiresAt         any              `json:"expires_at"` // RFC3339 string | null
	MaxViews          int              `json:"max_views"`
	ViewsServed       int              `json:"views_served"`
	HasAccessPassword bool             `json:"has_access_password"`
	Links             linkbuilder.Links `json:"links"`
	// Detail-only (omitempty so list 不膨胀)
	MIME     string `json:"mime,omitempty"`
	UploadIP string `json:"upload_ip,omitempty"`
	// Share-only
	ShareURL          string `json:"share_url,omitempty"`
	PasswordRequired  *bool  `json:"password_required,omitempty"`
}

func imageItemDTOFrom(row *imagesvc.Row, base string) ImageItemDTO {
	links := imageLinksFrom(base, &row.Img)
	var expiresAt any
	if row.Img.ExpiresAt != nil {
		expiresAt = row.Img.ExpiresAt.UTC().Format(time.RFC3339)
	}
	var slug any
	if row.Img.Slug != nil {
		slug = *row.Img.Slug
	}
	return ImageItemDTO{
		Key:               row.Img.Key,
		Slug:              slug,
		Name:              row.Img.Name,
		Ext:               row.Img.Ext,
		Size:              row.File.Size,
		Width:             row.File.Width,
		Height:            row.File.Height,
		Visibility:        row.Img.Visibility,
		AlbumID:           row.Img.AlbumID,
		CreatedAt:         row.Img.CreatedAt.Format(time.RFC3339),
		ExpiresAt:         expiresAt,
		MaxViews:          row.Img.MaxViews,
		ViewsServed:       row.Img.ViewsServed,
		HasAccessPassword: strings.TrimSpace(row.Img.AccessPasswordHash) != "",
		Links:             links,
	}
}
