package handler

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"gorm.io/gorm"

	"github.com/yixian-huang/imgli/internal/imaging"
)

const watermarkMaxBytes = 2 << 20

// UploadWatermark POST /api/v1/user/watermark(multipart file)。
// PNG ≤2MB 且 ≤2048×2048 原样存 <WatermarkDir>/<uid>.png(临时文件+rename 原子覆盖)。
func (h *UserHandlers) UploadWatermark(w http.ResponseWriter, r *http.Request) {
	u := PrincipalFrom(r).User
	// 整体 body 上限留 1MB 信封余量,文件本体在下方按字节单独限 2MB——
	// 避免 multipart 边界/头开销挤掉恰好 2MB 的合法文件(codex 终审)。
	r.Body = http.MaxBytesReader(w, r.Body, watermarkMaxBytes+1<<20)
	file, _, err := r.FormFile("file")
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			Fail(w, http.StatusRequestEntityTooLarge, CodeFileTooLarge, "水印图超过 2MB")
			return
		}
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "缺少 file 字段")
		return
	}
	defer file.Close()
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	data, err := io.ReadAll(io.LimitReader(file, watermarkMaxBytes+1))
	if err != nil {
		Fail(w, http.StatusInternalServerError, CodeInternal, "接收文件失败")
		return
	}
	if len(data) > watermarkMaxBytes {
		Fail(w, http.StatusRequestEntityTooLarge, CodeFileTooLarge, "水印图超过 2MB")
		return
	}
	meta, err := imaging.NewGo().Probe(bytes.NewReader(data))
	if err != nil || meta.Ext != "png" || meta.Width > 2048 || meta.Height > 2048 {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "水印图需为 PNG 且不超过 2048×2048")
		return
	}
	if err := os.MkdirAll(h.WatermarkDir, 0o755); err != nil {
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	dst := filepath.Join(h.WatermarkDir, fmt.Sprintf("%d.png", u.ID))
	tmp, err := os.CreateTemp(h.WatermarkDir, "watermark-*")
	if err != nil {
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	tmp.Close()
	if err := os.Rename(tmp.Name(), dst); err != nil {
		os.Remove(tmp.Name())
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	if err := h.Svc.SetWatermarkPath(u.ID, fmt.Sprintf("watermarks/%d.png", u.ID)); err != nil {
		// 元数据没落库(含用户已在并发注销中消失的 0 行更新):撤掉刚上线的文件,
		// 不留 DB 未确认的水印(codex 终审)。
		_ = os.Remove(dst)
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	OK(w, nil)
}

// DeleteWatermark DELETE /api/v1/user/watermark。幂等:用户行已消失(并发注销)视为已清除。
func (h *UserHandlers) DeleteWatermark(w http.ResponseWriter, r *http.Request) {
	u := PrincipalFrom(r).User
	_ = os.Remove(filepath.Join(h.WatermarkDir, fmt.Sprintf("%d.png", u.ID)))
	if err := h.Svc.SetWatermarkPath(u.ID, ""); err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	OK(w, nil)
}
