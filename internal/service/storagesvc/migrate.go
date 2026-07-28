package storagesvc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"gorm.io/gorm"

	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/storage"
)

// MigrateOpts 跨存储策略搬迁 file 行及其对象键（含缩略图）。
// 路径(file.Path)不变，仅改 storage_policy_id；适合 local→S3 同键复制。
type MigrateOpts struct {
	FromPolicyID uint64
	ToPolicyID   uint64
	// DryRun 只统计将迁移的条数，不写盘、不改库。
	DryRun bool
	// DeleteSource 成功复制并改库后删除源策略上的对象（含 .thumbs）。
	DeleteSource bool
	// Limit 最多处理 N 条；0=不限。
	Limit int
}

// MigrateResult 搬迁汇总。
type MigrateResult struct {
	Scanned int
	Copied  int
	Skipped int // 已在目标策略 / 源对象缺失
	Failed  int
	// Errors 至多保留前 20 条可读错误，防日志爆炸。
	Errors []string
}

// MigrateFiles 将 from 策略上的 files 复制到 to 策略并改指向。
// 缩略图：.thumbs/{hash}.jpg 与 .webp 若存在则一并复制（缺失不失败）。
func (r *Resolver) MigrateFiles(ctx context.Context, db *gorm.DB, opts MigrateOpts) (MigrateResult, error) {
	var out MigrateResult
	if opts.FromPolicyID == 0 || opts.ToPolicyID == 0 {
		return out, errors.New("storagesvc: from/to policy id 必填")
	}
	if opts.FromPolicyID == opts.ToPolicyID {
		return out, errors.New("storagesvc: from 与 to 不能相同")
	}
	var fromP, toP model.StoragePolicy
	if err := db.First(&fromP, opts.FromPolicyID).Error; err != nil {
		return out, fmt.Errorf("源策略: %w", err)
	}
	if err := db.First(&toP, opts.ToPolicyID).Error; err != nil {
		return out, fmt.Errorf("目标策略: %w", err)
	}
	if !toP.Enabled {
		return out, fmt.Errorf("目标策略 %d 未启用", toP.ID)
	}
	src, err := r.Driver(&fromP)
	if err != nil {
		return out, fmt.Errorf("源驱动: %w", err)
	}
	dst, err := r.Driver(&toP)
	if err != nil {
		return out, fmt.Errorf("目标驱动: %w", err)
	}

	q := db.Model(&model.File{}).Where("storage_policy_id = ?", opts.FromPolicyID).Order("id")
	if opts.Limit > 0 {
		q = q.Limit(opts.Limit)
	}
	var files []model.File
	if err := q.Find(&files).Error; err != nil {
		return out, err
	}

	for i := range files {
		out.Scanned++
		f := &files[i]
		if err := ctx.Err(); err != nil {
			out.addErr(fmt.Sprintf("file#%d: %v", f.ID, err))
			out.Failed++
			return out, err
		}
		if opts.DryRun {
			// 源对象存在性探测：不存在则记 skipped
			ok, e := src.Exists(ctx, f.Path)
			if e != nil {
				out.Failed++
				out.addErr(fmt.Sprintf("file#%d exists: %v", f.ID, e))
				continue
			}
			if !ok {
				out.Skipped++
				continue
			}
			out.Copied++ // dry-run 语义：将复制
			continue
		}
		if err := copyObject(ctx, src, dst, f.Path); err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				out.Skipped++
				slog.Warn("migrate 跳过缺失源对象", "file_id", f.ID, "path", f.Path)
				continue
			}
			out.Failed++
			out.addErr(fmt.Sprintf("file#%d put %s: %v", f.ID, f.Path, err))
			continue
		}
		// 缩略图 best-effort：现行世代 + 遗留键一并搬
		for _, tk := range ThumbKeyCandidates(f.Surface, f.Hash) {
			if err := copyObject(ctx, src, dst, tk); err != nil && !errors.Is(err, storage.ErrNotFound) {
				slog.Warn("migrate 缩略图失败(不阻断)", "file_id", f.ID, "key", tk, "err", err)
			}
		}
		res := db.Model(&model.File{}).
			Where("id = ? AND storage_policy_id = ?", f.ID, opts.FromPolicyID).
			Update("storage_policy_id", opts.ToPolicyID)
		if res.Error != nil {
			out.Failed++
			out.addErr(fmt.Sprintf("file#%d update: %v", f.ID, res.Error))
			continue
		}
		if res.RowsAffected == 0 {
			// 并发已迁走
			out.Skipped++
			continue
		}
		out.Copied++
		if opts.DeleteSource {
			_ = src.Delete(ctx, f.Path)
			for _, tk := range ThumbKeyCandidates(f.Surface, f.Hash) {
				_ = src.Delete(ctx, tk)
			}
		}
	}
	return out, nil
}

func copyObject(ctx context.Context, src, dst storage.Driver, key string) error {
	rc, err := src.Open(ctx, key)
	if err != nil {
		return err
	}
	defer rc.Close()
	// 确保从开头读（部分驱动 Seek 支持）
	if _, err := rc.Seek(0, io.SeekStart); err != nil {
		// 不可 seek 时假设已在起点
		_ = err
	}
	return dst.Put(ctx, key, rc)
}

func (r *MigrateResult) addErr(msg string) {
	if len(r.Errors) < 20 {
		r.Errors = append(r.Errors, msg)
	}
}
