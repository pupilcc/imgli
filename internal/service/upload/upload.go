// Package upload 实现上传管线：校验→去重秒传→落盘→建记录→缩略图。
package upload

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"

	"github.com/yixian-huang/imgli/internal/imaging"
	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/bandwidth"
	"github.com/yixian-huang/imgli/internal/service/moderation"
	"github.com/yixian-huang/imgli/internal/service/settings"
	"github.com/yixian-huang/imgli/internal/service/storagesvc"
	"github.com/yixian-huang/imgli/internal/storage"
	"github.com/yixian-huang/imgli/internal/task"
)

const (
	ThumbMaxEdge = 400
	MaxDimension = 30000 // 单边像素上限，防解压炸弹
)

var (
	ErrExtNotAllowed     = errors.New("upload: 文件类型不被允许")
	ErrFileTooLarge      = errors.New("upload: 文件超过大小上限")
	ErrQuotaExceeded     = errors.New("upload: 存储配额不足")
	ErrBandwidthExceeded = errors.New("upload: 本月流量已用尽")
	ErrDimensionOver     = errors.New("upload: 图片尺寸过大")
	ErrInvalidImage      = errors.New("upload: 不是有效图片")
	ErrGuestNotSupported = errors.New("upload: 游客上传暂未开放")
	ErrPolicyNotAllowed  = errors.New("upload: 存储策略不可用")
	ErrAlbumNotFound     = errors.New("upload: 相册不存在")
)

// Opts 上传选项。零值=完全走用户偏好与组默认。
// AlbumID 三态:nil=未指定(回退偏好);指向 0=明确不归档;指向 N=指定相册。
// ExpiresAt 非 nil 时写入 image.expires_at（由 handler 据 expires_in 用后端 time.Now() 算权威时间）。
type Opts struct {
	Visibility string
	AlbumID    *uint64
	PolicyID   uint64 // 0=未指定(回退偏好→组默认)
	ExpiresAt  *time.Time
}

type Result struct {
	Image   *model.Image
	File    *model.File
	Policy  *model.StoragePolicy
	Instant bool
}

type Service struct {
	db   *gorm.DB
	res  *storagesvc.Resolver
	proc imaging.Processor
	run  *task.Runner

	// WatermarkDir 用户水印图目录 <data_dir>/watermarks(D-② 烧录管线;空=用户水印不可用)。
	WatermarkDir string
}

func New(db *gorm.DB, res *storagesvc.Resolver, proc imaging.Processor, run *task.Runner) *Service {
	return &Service{db: db, res: res, proc: proc, run: run}
}

// GuestEnabled 返回 guest_upload_enabled 开关当前值；键缺失或读取异常一律按 false
// 处理（fail closed）。异常（非"键不存在"）会记一条 slog.Warn，便于运维区分"确实
// 关闭"与"设置服务读取故障"。供 Save 的权威门禁与 handler 层的前置门禁共用
// （handler 侧提前挡掉匿名请求、避免关闭时还去抓取/落盘再拒绝，见 upload.go）。
func (s *Service) GuestEnabled() bool {
	var enabled bool
	if err := settings.New(s.db).Get(model.SettingGuestUpload, &enabled); err != nil && !errors.Is(err, settings.ErrNotFound) {
		slog.Warn("读取 guest_upload_enabled 失败", "err", err)
	}
	return enabled
}

// Save 处理一个已落盘的临时文件。临时文件在返回前会被删除。
func (s *Service) Save(ctx context.Context, tmpPath, filename string, u *model.User, opts Opts, ip string) (*Result, error) {
	defer os.Remove(tmpPath)
	if u != nil && u.ID == 0 {
		u = nil // 规整零值 ID：使下文 u==nil 的游客判定口径统一
	}

	filename = truncateName(filename)

	// 用户组：配额、单文件上限、允许后缀、可用策略
	var group model.UserGroup
	if u == nil {
		if !s.GuestEnabled() {
			return nil, ErrGuestNotSupported
		}
		if err := s.db.Where("is_guest = ?", true).First(&group).Error; err != nil {
			return nil, err
		}
	} else {
		if err := s.db.First(&group, u.GroupID).Error; err != nil {
			return nil, err
		}
	}

	fi, err := os.Stat(tmpPath)
	if err != nil {
		return nil, err
	}
	size := fi.Size()
	if group.MaxFileSize > 0 && size > group.MaxFileSize {
		return nil, ErrFileTooLarge
	}

	// 探测元信息（独立打开）
	meta, err := s.probe(tmpPath)
	if err != nil {
		return nil, ErrInvalidImage
	}
	if meta.Width > MaxDimension || meta.Height > MaxDimension {
		return nil, ErrDimensionOver
	}
	if !extAllowed(group.AllowedExts, meta.Ext) {
		return nil, ErrExtNotAllowed
	}

	// D-② 处理管线:缩放→站点文字印→用户图片印,hash 之前烧录(与秒传去重天然兼容)。
	// 仅 jpg/jpeg/png;无处理项生效时字节完全不动。
	if changed, perr := s.burn(tmpPath, meta.Ext, u); perr != nil {
		return nil, perr
	} else if changed {
		if meta, err = s.probe(tmpPath); err != nil { // 尺寸可能已变,重探
			return nil, ErrInvalidImage
		}
		// 重编码改变了字节数:刷新 size,否则 files.size/配额累加/秒传计费全部用
		// 烧录前旧值长期漂移(codex 评审 Task4)。MaxFileSize 不复检——上限约束的是
		// 用户输入,站点加工的增量不归咎用户。
		fi2, serr := os.Stat(tmpPath)
		if serr != nil {
			return nil, serr
		}
		size = fi2.Size()
	}

	// 内容哈希（独立打开）
	hash, err := s.hashFile(tmpPath)
	if err != nil {
		return nil, err
	}

	// 可见性回退链:显式 > 偏好 > public(visibilityFor 内游客恒 public)
	vis := opts.Visibility
	if vis == "" && u != nil {
		vis = u.Preferences.DefaultVisibility
	}

	// 相册回退链(游客忽略):显式(0=明确不归档) > 偏好;显式无效 400,偏好悬空静默忽略
	albumID, err := s.resolveAlbum(u, opts.AlbumID)
	if err != nil {
		return nil, err
	}

	// 策略回退链:显式(须组内 enabled) > 偏好(悬空静默降级) > 组默认
	policy, err := s.resolvePolicy(&group, u, opts.PolicyID)
	if err != nil {
		return nil, err
	}

	// 配额：按 image 记录计，秒传与新传一视同仁（spec §6：去重省的是站点磁盘，不是用户配额）。
	// 快速预检减少无谓落盘；事务内 addUsedStorage 是权威门禁，堵住并发 TOCTOU。
	// 游客无账号、不计配额（游客组 storage_quota=0），跳过读取避免以 id=0 误查。
	if u != nil && group.StorageQuota > 0 {
		var freshUsed int64
		if err := s.db.Model(&model.User{}).Where("id = ?", u.ID).
			Select("used_storage").Scan(&freshUsed).Error; err != nil {
			return nil, err
		}
		if freshUsed+size > group.StorageQuota {
			return nil, ErrQuotaExceeded
		}
	}
	// 月流量硬顶：到顶禁上传（出图侧 serve 另拦）。
	if u != nil {
		if err := bandwidth.Check(s.db, u.ID); errors.Is(err, bandwidth.ErrExceeded) {
			return nil, ErrBandwidthExceeded
		} else if err != nil {
			return nil, err
		}
	}

	// 秒传：按 (hash, surface) 命中——私密只与私密去重，撞公开字节则走落盘分支复制独立私密对象。
	surface := visibilityFor(u, vis)
	var existing model.File
	hit := s.db.First(&existing, "hash = ? AND surface = ?", hash, surface).Error == nil
	if hit {
		img, err := s.commitInstant(u, &existing, filename, meta.Ext, vis, ip, size, albumID, opts.ExpiresAt, group.StorageQuota)
		if err != nil {
			return nil, err
		}
		// 只要没继承到 rejected/pending 实结论就自审:机审先写 score 再改 status(两次独立写),
		// "有分" 可能是机审中途(score 已写、status 未改),据此跳过会让新图永久停在 normal。
		// 仅继承到 rejected/pending(status 非 normal)才跳过——那才是真定论。
		if img.Status == "normal" {
			s.enqueueModerate(img.ID)
		}
		// 秒传复用既有文件所在策略生成链接(架构 spec「全局去重优先」裁决)——本次
		// 解析出的目标策略只约束新落盘;查不到时退回解析结果兜底(codex 评审 Task4)。
		var filePolicy model.StoragePolicy
		if err := s.db.First(&filePolicy, existing.StoragePolicyID).Error; err == nil {
			policy = &filePolicy
		}
		return &Result{Image: img, File: &existing, Policy: policy, Instant: true}, nil
	}

	// 落盘：渲染路径 → 驱动 Put
	driver, err := s.res.Driver(policy)
	if err != nil {
		return nil, err
	}
	relPath, err := s.res.RenderPath(policy.PathTemplate, meta.Ext, time.Now())
	if err != nil {
		return nil, err
	}
	relPath = storagesvc.SurfacePrefix(surface) + relPath // surface 前缀由写入代码加,不进模板
	if err := s.putFile(ctx, driver, relPath, tmpPath); err != nil {
		return nil, err
	}

	// 缩略图（同步）：失败不阻断上传——生产可接受缩略图缺失后台补
	thumb, terr := s.thumbnail(tmpPath)

	// 事务：建 file + image + 累加 used_storage
	// 内容安全 M-A：新建分支跨 surface 按 hash 继承同字节已有的最严审核结论——
	// scoped-dedup 下私密撞公开(或反之)字节会新建独立 File,不继承则已拒/待审内容
	// 会以新 surface 复活为 normal(绕过 commitInstant 的同 file 继承)。
	inhStatus, inhScore := inheritModerationByHash(s.db, hash)
	img := &model.Image{
		Name: filename, Ext: meta.Ext, Visibility: visibilityFor(u, vis),
		Status: inhStatus, NSFWScore: inhScore, UploadIP: ip, AlbumID: albumID, ExpiresAt: opts.ExpiresAt,
	}
	if u != nil {
		img.UserID = &u.ID
	}
	file := &model.File{
		Hash: hash, Surface: surface, StoragePolicyID: policy.ID, Path: relPath,
		Size: size, MIME: meta.MIME, Width: meta.Width, Height: meta.Height, RefCount: 1,
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(file).Error; err != nil {
			return err
		}
		img.FileID = file.ID
		key, err := s.uniqueKey(tx)
		if err != nil {
			return err
		}
		img.Key = key
		if err := tx.Create(img).Error; err != nil {
			return err
		}
		if u == nil {
			return nil // 游客不计配额，不累加 used_storage
		}
		return addUsedStorage(tx, u.ID, size, group.StorageQuota)
	})
	if err != nil {
		// 回滚：物理文件已落盘但记录失败 → 投递删除任务
		s.enqueueDelete(policy.ID, relPath)
		return nil, err
	}

	// 缩略图落盘（事务外，按文件哈希寻址：去重图共享同一缩略图）
	if terr == nil && thumb != nil {
		thumbKey := storagesvc.ThumbKey(surface, hash)
		if s.proc.ThumbExt() == "webp" {
			thumbKey = storagesvc.ThumbKeyWebP(surface, hash) // vips 构建产 WebP,键随格式(D-② 双探测对齐)
		}
		if err := driver.Put(ctx, thumbKey, bytes.NewReader(thumb)); err != nil {
			// 缩略图失败不影响主流程，直链 /t 会 404，可接受
			_ = err
		}
	}
	// 只要没继承到 rejected/pending 实结论就自审:机审先写 score 再改 status(两次独立写),
	// "有分" 可能是机审中途(score 已写、status 未改),据此跳过会让新图永久停在 normal。
	// 仅继承到 rejected/pending(status 非 normal)才跳过——那才是真定论;唯一 hash 的
	// 普通上传(无 sibling)仍 normal 且无分,照常入队。
	if img.Status == "normal" {
		s.enqueueModerate(img.ID)
	}
	return &Result{Image: img, File: file, Policy: policy, Instant: false}, nil
}

// burn 就地处理临时文件(缩放→站点文字水印→用户图片水印),返回是否发生改写。
// 仅 jpg/jpeg/png。settings 仅「键不存在」按默认降级,其余读取错误上抛中止——
// DB 故障静默全关会绕过已启用的站点水印策略(codex 评审 Task4)。
// 处理链任一步 ErrUnsupported(内容与扩展名不符等)→ 整链放弃、原字节不动、
// 记 slog.Warn:不写回部分处理的中间结果(原子降级语义,codex 评审 Task4)。
func (s *Service) burn(tmpPath, ext string, u *model.User) (bool, error) {
	switch strings.ToLower(ext) {
	case "jpg", "jpeg", "png":
	default:
		return false, nil
	}
	var proc Processing
	if err := settings.New(s.db).Get(model.SettingProcessing, &proc); err != nil {
		if !errors.Is(err, settings.ErrNotFound) {
			return false, err
		}
		proc = DefaultProcessing()
	}
	// 用户图片水印可用性:登录用户 + 偏好开启 + 目录已装配 + 文件存在
	var markData []byte
	if u != nil && u.Preferences.Watermark.Enabled && s.WatermarkDir != "" {
		if b, err := os.ReadFile(filepath.Join(s.WatermarkDir, fmt.Sprintf("%d.png", u.ID))); err == nil {
			markData = b
		}
	}
	textOn := proc.TextWatermark.Enabled && strings.TrimSpace(proc.TextWatermark.Text) != ""
	stripOn := proc.StripExifEnabled()
	if proc.MaxEdge == 0 && !textOn && markData == nil && !stripOn {
		return false, nil // 无处理项:字节完全不动
	}

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return false, err
	}
	out := data
	// EXIF 剥离放在缩放/水印前：先去元数据再处理；仅剥离子时也走重编码路径。
	if stripOn {
		v, err := imaging.StripMetadata(out)
		if errors.Is(err, imaging.ErrUnsupported) {
			slog.Warn("处理管线整链跳过(内容不可解码)", "step", "strip_exif")
			return false, nil
		}
		if err != nil {
			return false, err
		}
		out = v
	}
	if proc.MaxEdge > 0 {
		v, err := imaging.Scale(out, proc.MaxEdge)
		if errors.Is(err, imaging.ErrUnsupported) {
			slog.Warn("处理管线整链跳过(内容不可解码)", "step", "scale")
			return false, nil
		}
		if err != nil {
			return false, err
		}
		out = v
	}
	if textOn {
		v, err := imaging.WatermarkText(out, proc.TextWatermark.Text,
			proc.TextWatermark.Position, proc.TextWatermark.Opacity, proc.TextWatermark.SizeRatio)
		if errors.Is(err, imaging.ErrUnsupported) {
			slog.Warn("处理管线整链跳过(内容不可解码)", "step", "text_watermark")
			return false, nil
		}
		if err != nil {
			return false, err
		}
		out = v
	}
	if markData != nil {
		w := u.Preferences.Watermark
		pos := w.Position
		if pos == "" {
			pos = "br"
		}
		op := w.Opacity
		if op == 0 {
			op = 0.5
		}
		v, err := imaging.WatermarkImage(out, markData, pos, op, w.Margin)
		if errors.Is(err, imaging.ErrUnsupported) {
			// mark 文件坏/非 PNG:仅此步跳过不弃整链——底图处理(缩放/文字印)已确定可解码,
			// 弃链会连站点策略一起丢
			slog.Warn("处理管线跳过用户水印(水印图不可用)", "user_id", u.ID)
		} else if err != nil {
			return false, err
		} else {
			out = v
		}
	}
	if bytes.Equal(out, data) {
		return false, nil
	}
	// 覆写同一 tmpPath(0600 与 CreateTemp 缺省一致)
	if err := os.WriteFile(tmpPath, out, 0o600); err != nil {
		return false, err
	}
	return true, nil
}

// enqueueModerate 投递机审任务。是否 enabled 仍在任务执行时判断；此处按
// login_sample_rate 做登录用户抽检（游客全审）。runner 可能为 nil（单测）。
func (s *Service) enqueueModerate(imageID uint64) {
	if s.run == nil {
		return
	}
	// 读图拿 key/user 做抽检；失败则入队（fail-open 到机审侧）
	var img model.Image
	if err := s.db.Select("id", "key", "user_id").First(&img, imageID).Error; err == nil {
		cfg := moderation.DefaultConfig()
		if err := settings.New(s.db).Get(model.SettingModeration, &cfg); err != nil && !errors.Is(err, settings.ErrNotFound) {
			slog.Warn("enqueueModerate 读配置失败，仍入队", "err", err)
		} else {
			moderation.NormalizeConfig(&cfg)
			isGuest := img.UserID == nil
			if !moderation.ShouldEnqueueModerate(isGuest, cfg.LoginSampleRate, img.Key) {
				slog.Info("机审抽检跳过", "image_id", imageID, "key", img.Key, "rate", cfg.LoginSampleRate)
				return
			}
		}
	}
	payload, _ := json.Marshal(map[string]any{"image_id": imageID})
	if err := s.run.Enqueue("moderate_image", string(payload)); err != nil {
		slog.Error("投递 moderate_image 任务失败", "image_id", imageID, "err", err)
	}
}

func (s *Service) commitInstant(u *model.User, file *model.File, filename, ext, visibility, ip string, size int64, albumID *uint64, expiresAt *time.Time, storageQuota int64) (*model.Image, error) {
	// 内容安全 P1：秒传继承同 file 上已有 image 的审核态与最高 nsfw_score，
	// 防止 rejected/pending 脏 hash 以新 key 复活为 normal。
	status, score := inheritModerationFrom(s.db, file.ID)
	img := &model.Image{
		Name: filename, Ext: ext, Visibility: visibilityFor(u, visibility),
		Status: status, NSFWScore: score, UploadIP: ip, FileID: file.ID, AlbumID: albumID,
		ExpiresAt: expiresAt,
	}
	if u != nil {
		img.UserID = &u.ID
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		key, err := s.uniqueKey(tx)
		if err != nil {
			return err
		}
		img.Key = key
		if err := tx.Create(img).Error; err != nil {
			return err
		}
		// 条件 +1：file 若已被 purge 删行则 0 行，回滚秒传（防 purge-vs-秒传 竞态）
		res := tx.Model(&model.File{}).Where("id = ?", file.ID).
			UpdateColumn("ref_count", gorm.Expr("ref_count + 1"))
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		file.RefCount++
		if u == nil {
			return nil // 游客秒传不计配额，不累加 used_storage
		}
		return addUsedStorage(tx, u.ID, size, storageQuota)
	})
	if err != nil {
		return nil, err
	}
	return img, nil
}

// addUsedStorage 事务内原子累加配额。quota>0 时用 used_storage+size<=quota 条件更新，
// 堵住双请求均通过预检后超配额的 TOCTOU；quota<=0 表示不限。
func addUsedStorage(tx *gorm.DB, userID uint64, size, quota int64) error {
	var res *gorm.DB
	if quota > 0 {
		res = tx.Model(&model.User{}).
			Where("id = ? AND used_storage + ? <= ?", userID, size, quota).
			UpdateColumn("used_storage", gorm.Expr("used_storage + ?", size))
	} else {
		res = tx.Model(&model.User{}).Where("id = ?", userID).
			UpdateColumn("used_storage", gorm.Expr("used_storage + ?", size))
	}
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		// 区分：用户已注销 vs 配额不足
		var n int64
		if err := tx.Model(&model.User{}).Where("id = ?", userID).Count(&n).Error; err != nil {
			return err
		}
		if n == 0 {
			return gorm.ErrRecordNotFound
		}
		return ErrQuotaExceeded
	}
	return nil
}

// inheritModerationFrom 汇总同 file_id 既有 image 的审核结论。
// 严重度 rejected > pending > normal；nsfw_score 取非空最大值。
// 软删图不参与（gorm 默认 scope）。无兄弟图时返回 normal / nil。
func inheritModerationFrom(db *gorm.DB, fileID uint64) (status string, score *float64) {
	status = "normal"
	var siblings []model.Image
	if err := db.Select("status", "nsfw_score").Where("file_id = ?", fileID).Find(&siblings).Error; err != nil {
		return status, nil
	}
	for _, sib := range siblings {
		switch sib.Status {
		case "rejected":
			status = "rejected"
		case "pending":
			if status != "rejected" {
				status = "pending"
			}
		}
		if sib.NSFWScore != nil {
			if score == nil || *sib.NSFWScore > *score {
				v := *sib.NSFWScore
				score = &v
			}
		}
	}
	return status, score
}

// inheritModerationByHash 跨 surface 汇总同 hash 所有 File 上已有 image 的审核态与最高分。
// 用于上传新建分支:scoped-dedup 下私密撞公开(或反之)字节会新建独立 File,若不继承已有
// rejected/pending 结论,已拒内容会以新 surface 复活为 normal(绕过 commitInstant 的同 file
// 继承)。归并逻辑与 inheritModerationFrom 一致,只是范围扩到同 hash 的全部 File。
func inheritModerationByHash(db *gorm.DB, hash string) (status string, score *float64) {
	status = "normal"
	var fileIDs []uint64
	if err := db.Model(&model.File{}).Where("hash = ?", hash).Pluck("id", &fileIDs).Error; err != nil || len(fileIDs) == 0 {
		return status, nil
	}
	var siblings []model.Image
	if err := db.Select("status", "nsfw_score").Where("file_id IN ?", fileIDs).Find(&siblings).Error; err != nil {
		return status, nil
	}
	for _, sib := range siblings {
		switch sib.Status {
		case "rejected":
			status = "rejected"
		case "pending":
			if status != "rejected" {
				status = "pending"
			}
		}
		if sib.NSFWScore != nil {
			if score == nil || *sib.NSFWScore > *score {
				v := *sib.NSFWScore
				score = &v
			}
		}
	}
	return status, score
}

// resolveAlbum 解析相册三态。游客(u==nil)恒 nil。
func (s *Service) resolveAlbum(u *model.User, explicit *uint64) (*uint64, error) {
	if u == nil {
		return nil, nil
	}
	if explicit != nil {
		if *explicit == 0 {
			return nil, nil // 明确不归档,不回退偏好
		}
		var n int64
		if err := s.db.Model(&model.Album{}).
			Where("id = ? AND user_id = ?", *explicit, u.ID).Count(&n).Error; err != nil {
			return nil, err
		}
		if n == 0 {
			return nil, ErrAlbumNotFound
		}
		return explicit, nil
	}
	if p := u.Preferences.DefaultAlbumID; p != nil {
		var n int64
		if err := s.db.Model(&model.Album{}).
			Where("id = ? AND user_id = ?", *p, u.ID).Count(&n).Error; err != nil {
			return nil, err
		}
		if n > 0 {
			return p, nil
		}
		// 偏好悬空(相册已删):静默忽略(spec 风险 1)
	}
	return nil, nil
}

// resolvePolicy 解析策略:显式(须在组 allowed 且 enabled,否则 ErrPolicyNotAllowed)>
// 偏好(悬空静默降级)> 组默认 pickPolicy。游客忽略显式与偏好。
func (s *Service) resolvePolicy(g *model.UserGroup, u *model.User, explicit uint64) (*model.StoragePolicy, error) {
	inAllowed := func(id uint64) bool {
		for _, a := range g.AllowedPolicyIDs {
			if a == id {
				return true
			}
		}
		return false
	}
	if u != nil && explicit != 0 {
		if !inAllowed(explicit) {
			return nil, ErrPolicyNotAllowed
		}
		var p model.StoragePolicy
		if err := s.db.First(&p, "id = ? AND enabled = ?", explicit, true).Error; err != nil {
			return nil, ErrPolicyNotAllowed
		}
		return &p, nil
	}
	if u != nil && u.Preferences.DefaultPolicyID != nil && inAllowed(*u.Preferences.DefaultPolicyID) {
		var p model.StoragePolicy
		if err := s.db.First(&p, "id = ? AND enabled = ?", *u.Preferences.DefaultPolicyID, true).Error; err == nil {
			return &p, nil
		}
	}
	return s.pickPolicy(g)
}

const base62 = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// randBase62 返回 n 位随机 base62 字符串。
func randBase62(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, n)
	for i := range buf {
		out[i] = base62[int(buf[i])%62]
	}
	return string(out), nil
}

func (s *Service) uniqueKey(tx *gorm.DB) (string, error) {
	for i := 0; i < 5; i++ {
		key, err := randBase62(12) // model.Image.Key: 12 位 base62，直链用
		if err != nil {
			return "", err
		}
		var n int64
		if err := tx.Unscoped().Model(&model.Image{}).Where("key = ?", key).Count(&n).Error; err != nil {
			return "", err
		}
		if n == 0 {
			return key, nil
		}
	}
	return "", errors.New("upload: 无法生成唯一 key")
}

func (s *Service) probe(p string) (imaging.Meta, error) {
	f, err := os.Open(p)
	if err != nil {
		return imaging.Meta{}, err
	}
	defer f.Close()
	return s.proc.Probe(f)
}

func (s *Service) thumbnail(p string) ([]byte, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return s.proc.Thumbnail(f, ThumbMaxEdge)
}

func (s *Service) hashFile(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (s *Service) putFile(ctx context.Context, d storage.Driver, key, srcPath string) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()
	return d.Put(ctx, key, f)
}

func (s *Service) pickPolicy(g *model.UserGroup) (*model.StoragePolicy, error) {
	var p model.StoragePolicy
	if len(g.AllowedPolicyIDs) > 0 {
		if err := s.db.First(&p, "id = ? AND enabled = ?", g.AllowedPolicyIDs[0], true).Error; err == nil {
			return &p, nil
		}
	}
	if err := s.db.First(&p, "enabled = ?", true).Error; err != nil {
		return nil, fmt.Errorf("upload: 无可用存储策略: %w", err)
	}
	return &p, nil
}

func (s *Service) enqueueDelete(policyID uint64, key string) {
	payload, _ := json.Marshal(map[string]any{"policy_id": policyID, "key": key})
	if err := s.run.Enqueue("delete_file", string(payload)); err != nil {
		slog.Error("投递 delete_file 回滚任务失败", "policy_id", policyID, "key", key, "err", err)
	}
}

// DeleteFileTask 删除物理文件（供 task runner 注册）。payload = {"policy_id":N,"key":"..."}。
func (s *Service) DeleteFileTask(ctx context.Context, payload string) error {
	var p struct {
		PolicyID uint64 `json:"policy_id"`
		Key      string `json:"key"`
	}
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		return err
	}
	var policy model.StoragePolicy
	if err := s.db.First(&policy, p.PolicyID).Error; err != nil {
		return err
	}
	d, err := s.res.Driver(&policy)
	if err != nil {
		return err
	}
	return d.Delete(ctx, p.Key)
}

func extAllowed(allowed []string, ext string) bool {
	for _, a := range allowed {
		if strings.EqualFold(a, ext) {
			return true
		}
	}
	return len(allowed) == 0 // 空白名单视为不限（保守：应至少有默认后缀）
}

// truncateName 将展示名截断到 <=255 字节且不破坏 UTF-8 字符边界。
func truncateName(s string) string {
	const max = 255
	if len(s) <= max {
		return s
	}
	b := []byte(s)[:max]
	for len(b) > 0 && !utf8.Valid(b) {
		b = b[:len(b)-1]
	}
	return string(b)
}

func normVisibility(v string) string {
	if v == "private" {
		return "private"
	}
	return "public"
}

// visibilityFor 计算落库可见性：游客图（u==nil）恒 public，无视调用方传入的
// visibility——游客无账号，owner 校验永远匹配不到 NULL user_id，一旦允许游客建
// private 记录就会成为谁也看不到的死记录（spec §5 "游客 visibility 恒 public"）。
func visibilityFor(u *model.User, visibility string) string {
	if u == nil {
		return "public"
	}
	return normVisibility(visibility)
}
