package imagesvc

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/storagesvc"
	"github.com/yixian-huang/imgli/internal/storage"
)

// copyObjectKey 把 src 键对象复制到 dst 键。读入内存再 Put:s3 Put 需 Content-Length
// (bodyLen 认 bytes.Reader.Len()),而 driver.Open 返回的 reader(s3 rangeReadSeekCloser)
// 无 Len()→chunked→S3 拒 MissingContentLength。切换罕见、对象 MB 级,缓冲可接受。
func copyObjectKey(ctx context.Context, d storage.Driver, src, dst string) error {
	rc, err := d.Open(ctx, src)
	if err != nil {
		return err
	}
	data, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		return err
	}
	return d.Put(ctx, dst, bytes.NewReader(data))
}

// copyThumbAcross 把 oldSurface 缩略图复制到 newSurface 同 ext 键。源按候选探测(含 public
// 遗留),命中即复制。仅当所有候选都明确不存在(ErrNotFound)才返 nil 容忍;读/写错误上抛,
// 由调用方中止重挂——防"源存在但复制失败仍提交"致新 surface /t 永久 404。

// copyThumbAcross 把 oldSurface 缩略图复制到 newSurface 同 ext 键。源按候选探测(含 public
// 遗留),命中即复制。仅当所有候选都明确不存在(ErrNotFound)才返 nil 容忍;读/写错误上抛,
// 由调用方中止重挂——防"源存在但复制失败仍提交"致新 surface /t 永久 404。
func (s *Service) copyThumbAcross(ctx context.Context, d storage.Driver, oldSurface, newSurface, hash string) error {
	for _, src := range storagesvc.ThumbKeyCandidates(oldSurface, hash) {
		rc, err := d.Open(ctx, src)
		if errors.Is(err, storage.ErrNotFound) {
			continue // 该候选确实不存在,试下一个
		}
		if err != nil {
			return err // 瞬时/网络错误 → 中止,不当作无缩略图
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return err
		}
		dst := storagesvc.ThumbKey(newSurface, hash)
		if strings.HasSuffix(src, ".webp") {
			dst = storagesvc.ThumbKeyWebP(newSurface, hash)
		}
		return d.Put(ctx, dst, bytes.NewReader(data))
	}
	return nil // 所有候选均不存在 → 无缩略图可复制,容忍
}

// resolveFileForSurface 找到或创建 targetSurface 上与 srcFile 同 hash 的 File。
// 命中:返回既有(调用方负责 ref++)。未命中:复制原图对象与缩略图到 targetSurface 前缀,
// 建 File 行(Surface=target, Path=新键, ref_count=0)返回。
// 非事务:复制走存储 I/O 不占 DB 锁;建行后若外层 ref 事务失败,新 File 为 ref-0 孤儿(可 GC)。

// resolveFileForSurface 找到或创建 targetSurface 上与 srcFile 同 hash 的 File。
// 命中:返回既有(调用方负责 ref++)。未命中:复制原图对象与缩略图到 targetSurface 前缀,
// 建 File 行(Surface=target, Path=新键, ref_count=0)返回。
// 非事务:复制走存储 I/O 不占 DB 锁;建行后若外层 ref 事务失败,新 File 为 ref-0 孤儿(可 GC)。
func (s *Service) resolveFileForSurface(policy *model.StoragePolicy, srcFile *model.File, targetSurface string) (*model.File, error) {
	var existing model.File
	err := s.db.First(&existing, "hash = ? AND surface = ?", srcFile.Hash, targetSurface).Error
	if err == nil {
		return &existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	d, err := s.res.Driver(policy)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	ext := strings.TrimPrefix(filepath.Ext(srcFile.Path), ".")
	relPath, err := s.res.RenderPath(policy.PathTemplate, ext, time.Now())
	if err != nil {
		return nil, err
	}
	newPath := storagesvc.SurfacePrefix(targetSurface) + relPath
	if err := copyObjectKey(ctx, d, srcFile.Path, newPath); err != nil {
		return nil, err
	}
	if err := s.copyThumbAcross(ctx, d, srcFile.Surface, targetSurface, srcFile.Hash); err != nil {
		_ = d.Delete(ctx, newPath) // 补偿:缩略图失败中止,删掉已复制的原图对象,防孤儿泄漏
		return nil, err
	}
	newFile := &model.File{
		Hash: srcFile.Hash, Surface: targetSurface, StoragePolicyID: srcFile.StoragePolicyID,
		Path: newPath, Size: srcFile.Size, MIME: srcFile.MIME, Width: srcFile.Width, Height: srcFile.Height,
		RefCount: 0,
	}
	if err := s.db.Create(newFile).Error; err != nil {
		// 并发未命中同 (hash,targetSurface):唯一键冲突 → 回查胜者返回(不依赖方言是否把
		// 约束错译成 gorm.ErrDuplicatedKey)。胜者已建时补偿删除我方已复制的 newPath 对象,防孤儿泄漏。
		var winner model.File
		if e := s.db.First(&winner, "hash = ? AND surface = ?", srcFile.Hash, targetSurface).Error; e == nil {
			_ = d.Delete(ctx, newPath) // 补偿:胜者已建,删我方孤儿原图对象
			return &winner, nil
		}
		// 非并发原因建行失败(DB/FK 等)且无胜者:已复制的 newPath 原图无 File 行、
		// 无删除任务,ref-0 清理找不到 → 补偿删除防孤儿(缩略图键确定性可能被并发共享,不删)。
		_ = d.Delete(ctx, newPath)
		return nil, err
	}
	return newFile, nil
}
