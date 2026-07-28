package handler

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"github.com/yixian-huang/imgli/internal/imaging"
)

const avatarMaxBytes = 5 << 20

// UploadAvatar POST /api/v1/user/avatar(multipart file)。
// 方裁 256 JPEG 存 <AvatarDir>/<uid>.jpg 固定名(临时文件+rename 原子覆盖)。
func (h *UserHandlers) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	u := PrincipalFrom(r).User
	// 整体 body 上限留 1MB 信封余量,文件本体在下方按字节单独限 5MB——
	// 避免 multipart 边界/头开销挤掉恰好 5MB 的合法文件(codex 终审)。
	r.Body = http.MaxBytesReader(w, r.Body, avatarMaxBytes+1<<20)
	file, _, err := r.FormFile("file")
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			Fail(w, http.StatusRequestEntityTooLarge, CodeFileTooLarge, "头像文件超过 5MB")
			return
		}
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "缺少 file 字段")
		return
	}
	defer file.Close()
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	data, err := io.ReadAll(io.LimitReader(file, avatarMaxBytes+1))
	if err != nil {
		Fail(w, http.StatusInternalServerError, CodeInternal, "接收文件失败")
		return
	}
	if len(data) > avatarMaxBytes {
		Fail(w, http.StatusRequestEntityTooLarge, CodeFileTooLarge, "头像文件超过 5MB")
		return
	}
	jpg, err := imaging.Avatar(data, 256)
	if err != nil {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "不是有效的图片文件")
		return
	}
	if err := os.MkdirAll(h.AvatarDir, 0o755); err != nil {
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	dst := filepath.Join(h.AvatarDir, fmt.Sprintf("%d.jpg", u.ID))
	tmp, err := os.CreateTemp(h.AvatarDir, "avatar-*")
	if err != nil {
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	if _, err := tmp.Write(jpg); err != nil {
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
	if err := h.Svc.SetAvatarPath(u.ID, fmt.Sprintf("avatars/%d.jpg", u.ID)); err != nil {
		// 元数据没落库(含用户已在并发注销中消失的 0 行更新):撤掉刚上线的公开文件,
		// 不留 DB 未确认的头像(codex 终审)。
		_ = os.Remove(dst)
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	OK(w, nil)
}

// DeleteAvatar DELETE /api/v1/user/avatar。幂等:用户行已消失(并发注销)视为已清除。
func (h *UserHandlers) DeleteAvatar(w http.ResponseWriter, r *http.Request) {
	u := PrincipalFrom(r).User
	_ = os.Remove(filepath.Join(h.AvatarDir, fmt.Sprintf("%d.jpg", u.ID)))
	if err := h.Svc.SetAvatarPath(u.ID, ""); err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	OK(w, nil)
}

// ServeAvatar GET /avatar/{id} 公开只读(未来广场复用)。无头像 404(纯 http,非信封)。
func ServeAvatar(dir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		p := filepath.Join(dir, fmt.Sprintf("%d.jpg", id))
		if _, err := os.Stat(p); err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=86400")
		http.ServeFile(w, r, p)
	}
}
