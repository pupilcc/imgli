// Package imagesvc 实现用户侧图片管理（列表/详情/变更/回收站/批量）。
package imagesvc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/storagesvc"
	"github.com/yixian-huang/imgli/internal/storage"
	"github.com/yixian-huang/imgli/internal/task"
)

var (
	ErrInvalidSort       = errors.New("imagesvc: 非法排序")
	ErrBadCursor         = errors.New("imagesvc: 非法游标")
	ErrInvalidFilter     = errors.New("imagesvc: 非法筛选参数")
	ErrNotFound          = errors.New("imagesvc: 图片不存在")
	ErrInvalidVisibility = errors.New("imagesvc: 可见性仅 public|private")
	ErrAlbumNotFound     = errors.New("imagesvc: 相册不存在")
	ErrInvalidName       = errors.New("imagesvc: 名称需 1-255 字节")
	ErrInvalidAction     = errors.New("imagesvc: 未知批量操作")
	ErrInvalidSlug       = errors.New("imagesvc: slug 需 3-32 位小写字母数字与连字符")
	ErrSlugTaken         = errors.New("imagesvc: slug 已被占用")
	slugRe               = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{1,30}[a-z0-9])?$`)
)

// errRehomeConflict 并发切换已把本图重挂到目标 surface——本次回滚 ref 变更,返回当前态。
var errRehomeConflict = errors.New("imagesvc: 并发重挂冲突")

// Filter 是列表筛选条件。空字段=不筛。
type Filter struct {
	Q, Format, Album, Visibility, Sort string
}

// Row 是一条图片连同其物理文件与存储策略（供构造链接与展示元信息）。
type Row struct {
	Img    model.Image
	File   model.File
	Policy model.StoragePolicy
}

type Service struct {
	db  *gorm.DB
	res *storagesvc.Resolver // 复制对象/取 driver 用（S2 surface 切换）
	run *task.Runner         // 可为 nil（服务级单测无需投递物理删除）
}

func New(db *gorm.DB, res *storagesvc.Resolver, run *task.Runner) *Service {
	return &Service{db: db, res: res, run: run}
}

// formatExts 把 UI 的 format chip 值映射为 ext 集合；空/ALL 返回 nil（不筛）。
func formatExts(format string) []string {
	switch strings.ToUpper(format) {
	case "", "ALL":
		return nil
	case "JPG", "JPEG":
		return []string{"jpg", "jpeg"}
	case "PNG":
		return []string{"png"}
	case "GIF":
		return []string{"gif"}
	case "WEBP":
		return []string{"webp"}
	default:
		return []string{strings.ToLower(format)}
	}
}

// orderSpec 返回 (排序列, 是否倒序)。未知返回 ok=false。
func orderSpec(sort string) (col string, desc, ok bool) {
	switch sort {
	case "", "date":
		// 用 images.id（单调自增、与 created_at 同步写入）做 keyset 代理，
		// 避免时间戳精度/类型在跨方言下与游标绑定值比较时失真（见 Finding 1）。
		return "images.id", true, true
	case "size":
		return "files.size", true, true
	case "name":
		return "images.name", false, true
	default:
		return "", false, false
	}
}

// applyFilters 加公共 where（不含软删/归属，由调用方先加）。
func applyFilters(q *gorm.DB, f Filter) (*gorm.DB, error) {
	if f.Q != "" {
		q = q.Where("images.name LIKE ?", "%"+f.Q+"%")
	}
	if exts := formatExts(f.Format); exts != nil {
		q = q.Where("images.ext IN ?", exts)
	}
	switch f.Album {
	case "":
		// 不筛
	case "none":
		q = q.Where("images.album_id IS NULL")
	default:
		id, err := strconv.ParseUint(f.Album, 10, 64)
		if err != nil {
			return nil, ErrInvalidFilter
		}
		q = q.Where("images.album_id = ?", id)
	}
	if f.Visibility == "public" || f.Visibility == "private" {
		q = q.Where("images.visibility = ?", f.Visibility)
	}
	return q, nil
}

// listScan 承接 images.* 加派生排序值。
type listScan struct {
	model.Image
	SortSize int64 `gorm:"column:sort_size"`
}

func (s *Service) List(userID uint64, f Filter, cursor string, limit int) ([]Row, string, error) {
	col, desc, ok := orderSpec(f.Sort)
	if !ok {
		return nil, "", ErrInvalidSort
	}
	if limit <= 0 {
		limit = 24
	}
	q := s.db.Table("images").
		Joins("JOIN files ON files.id = images.file_id").
		Where("images.user_id = ? AND images.deleted_at IS NULL", userID).
		Where("images.expires_at IS NULL OR images.expires_at > ?", time.Now())
	var err error
	if q, err = applyFilters(q, f); err != nil {
		return nil, "", err
	}
	if cursor != "" {
		cur, derr := decodeListCursor(cursor)
		if derr != nil || cur.Sort != normSort(f.Sort) {
			return nil, "", ErrBadCursor
		}
		cmp := "<"
		if !desc {
			cmp = ">"
		}
		// keyset: (col,id) 在边界之后 —— col cmp ? OR (col = ? AND images.id cmp ?)
		cond := fmt.Sprintf("%s %s ? OR (%s = ? AND images.id %s ?)", col, cmp, col, cmp)
		if f.Sort == "name" {
			q = q.Where(cond, cur.ValStr, cur.ValStr, cur.ID)
		} else {
			q = q.Where(cond, cur.ValInt, cur.ValInt, cur.ID)
		}
	}
	dir := "DESC"
	if !desc {
		dir = "ASC"
	}
	q = q.Select("images.*, files.size AS sort_size").
		Order(fmt.Sprintf("%s %s, images.id %s", col, dir, dir)).
		Limit(limit + 1)

	var scans []listScan
	if err := q.Scan(&scans).Error; err != nil {
		return nil, "", err
	}
	next := ""
	if len(scans) > limit {
		last := scans[limit-1]
		scans = scans[:limit]
		c := listCursor{Sort: normSort(f.Sort), ID: last.ID}
		switch f.Sort {
		case "size":
			c.ValInt = last.SortSize
		case "name":
			c.ValStr = last.Name
		default:
			c.ValInt = int64(last.ID)
		}
		next = encodeListCursor(c)
	}
	rows, err := s.hydrate(scans)
	if err != nil {
		return nil, "", err
	}
	return rows, next, nil
}

// Get 取属主单图（连 file/policy）。非属主或不存在返回 ErrNotFound。
func (s *Service) Get(userID uint64, key string) (*Row, error) {
	var img model.Image
	err := s.db.Where("key = ? AND user_id = ?", key, userID).First(&img).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var file model.File
	if err := s.db.First(&file, img.FileID).Error; err != nil {
		return nil, err
	}
	var pol model.StoragePolicy
	if err := s.db.First(&pol, file.StoragePolicyID).Error; err != nil {
		return nil, err
	}
	return &Row{Img: img, File: file, Policy: pol}, nil
}

func normSort(sort string) string {
	if sort == "" {
		return "date"
	}
	return sort
}

// hydrate 批量装载 File 与 StoragePolicy，避免 N+1。
func (s *Service) hydrate(scans []listScan) ([]Row, error) {
	if len(scans) == 0 {
		return []Row{}, nil
	}
	fileIDs := make([]uint64, 0, len(scans))
	for i := range scans {
		fileIDs = append(fileIDs, scans[i].FileID)
	}
	var files []model.File
	if err := s.db.Where("id IN ?", fileIDs).Find(&files).Error; err != nil {
		return nil, err
	}
	fileByID := map[uint64]model.File{}
	policyIDs := map[uint64]struct{}{}
	for _, f := range files {
		fileByID[f.ID] = f
		policyIDs[f.StoragePolicyID] = struct{}{}
	}
	pids := make([]uint64, 0, len(policyIDs))
	for id := range policyIDs {
		pids = append(pids, id)
	}
	var policies []model.StoragePolicy
	if err := s.db.Where("id IN ?", pids).Find(&policies).Error; err != nil {
		return nil, err
	}
	polByID := map[uint64]model.StoragePolicy{}
	for _, p := range policies {
		polByID[p.ID] = p
	}
	out := make([]Row, 0, len(scans))
	for i := range scans {
		f := fileByID[scans[i].FileID]
		out = append(out, Row{Img: scans[i].Image, File: f, Policy: polByID[f.StoragePolicyID]})
	}
	return out, nil
}

// Update 部分更新单图。name/visibility 为 nil 表示不改；albumID：nil=不改，0=移出，>0=移入(校验归属)。
// expiresAt/setExpires：setExpires=false 不改；true 时写入 expiresAt（nil 即清除为 NULL）。
// slug：nil=不改；""=清除；否则校验 [a-z0-9-]{3,32} 并唯一。
func (s *Service) Update(userID uint64, key string, name, visibility *string, albumID *int64, expiresAt *time.Time, setExpires bool, slug *string) (*Row, error) {
	var img model.Image
	err := s.db.Where("key = ? AND user_id = ?", key, userID).First(&img).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	updates := map[string]any{}
	if name != nil {
		n := strings.TrimSpace(*name)
		if n == "" || len(n) > 255 {
			return nil, ErrInvalidName
		}
		updates["name"] = n
	}
	if visibility != nil {
		if *visibility != "public" && *visibility != "private" {
			return nil, ErrInvalidVisibility
		}
		updates["visibility"] = *visibility
	}
	if albumID != nil {
		if *albumID == 0 {
			updates["album_id"] = nil
		} else {
			var cnt int64
			if err := s.db.Model(&model.Album{}).
				Where("id = ? AND user_id = ?", *albumID, userID).Count(&cnt).Error; err != nil {
				return nil, err
			}
			if cnt == 0 {
				return nil, ErrAlbumNotFound
			}
			updates["album_id"] = uint64(*albumID)
		}
	}
	if setExpires {
		// map 中显式写 nil → GORM Updates 写 NULL（与 album_id 清出同模式）
		updates["expires_at"] = expiresAt
	}
	if slug != nil {
		v := strings.ToLower(strings.TrimSpace(*slug))
		if v == "" {
			updates["slug"] = nil
		} else {
			if !slugRe.MatchString(v) {
				return nil, ErrInvalidSlug
			}
			var n int64
			if err := s.db.Model(&model.Image{}).Where("slug = ? AND key <> ?", v, key).Count(&n).Error; err != nil {
				return nil, err
			}
			if n > 0 {
				return nil, ErrSlugTaken
			}
			// 勿与既有 key 冲突
			if err := s.db.Model(&model.Image{}).Where("key = ? AND key <> ?", v, key).Count(&n).Error; err != nil {
				return nil, err
			}
			if n > 0 {
				return nil, ErrSlugTaken
			}
			updates["slug"] = v
		}
	}
	// 可见性变更 → surface 重挂(私密图对象层防护 S2):把 img 重挂到目标 surface 的 File,
	// 复制对象(事务外),调 ref_count,旧 File 归零投递异步 purge。surface == visibility。
	if visibility != nil && *visibility != img.Visibility {
		var oldFile model.File
		if err := s.db.First(&oldFile, img.FileID).Error; err != nil {
			return nil, err
		}
		var policy model.StoragePolicy
		if err := s.db.First(&policy, oldFile.StoragePolicyID).Error; err != nil {
			return nil, err
		}
		newFile, err := s.resolveFileForSurface(&policy, &oldFile, *visibility)
		if err != nil {
			return nil, err
		}
		updates["file_id"] = newFile.ID
		var pd *physicalDelete
		err = s.db.Transaction(func(tx *gorm.DB) error {
			// CAS 重挂:仅当 img 仍指向 oldFile 才重挂——防并发切换重复调 ref。
			// 先于 ref 调整与删旧 File:此后 img 不再引用 oldFile,删其行不违反
			// images.file_id 的 ON DELETE RESTRICT。
			res := tx.Model(&model.Image{}).Where("id = ? AND file_id = ?", img.ID, oldFile.ID).Updates(updates)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				return errRehomeConflict // 并发已重挂,回滚(本事务未做 ref 变更)
			}
			// newFile ref++(条件:防并发 purge 删行,同 upload 秒传先例)
			r1 := tx.Model(&model.File{}).Where("id = ?", newFile.ID).
				UpdateColumn("ref_count", gorm.Expr("ref_count + 1"))
			if r1.Error != nil {
				return r1.Error
			}
			if r1.RowsAffected == 0 {
				return gorm.ErrRecordNotFound
			}
			// oldFile ref--
			if err := tx.Model(&model.File{}).Where("id = ?", oldFile.ID).
				UpdateColumn("ref_count", gorm.Expr("ref_count - ?", 1)).Error; err != nil {
				return err
			}
			// 条件删:仅当减后 ≤0 才删旧 File 行 → 备物理删除旧对象(此时 img 已重挂)
			del := tx.Where("id = ? AND ref_count <= 0", oldFile.ID).Delete(&model.File{})
			if del.Error != nil {
				return del.Error
			}
			if del.RowsAffected == 1 {
				pd = &physicalDelete{
					policyID: oldFile.StoragePolicyID, path: oldFile.Path,
					thumbs: storagesvc.ThumbKeyCandidates(oldFile.Surface, oldFile.Hash),
				}
			}
			return nil
		})
		if errors.Is(err, errRehomeConflict) {
			return s.Get(userID, key) // 并发已达目标态,返回当前
		}
		if err != nil {
			return nil, err
		}
		s.enqueuePhysical(pd)
		return s.Get(userID, key)
	}

	if len(updates) > 0 {
		if err := s.db.Model(&img).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	return s.Get(userID, key)
}

// SoftDelete 软删（进回收站，保配额，直链转 410）。非属主→ErrNotFound。
func (s *Service) SoftDelete(userID uint64, key string) error {
	res := s.db.Where("key = ? AND user_id = ?", key, userID).Delete(&model.Image{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// BatchResult 单键批量结果。
type BatchResult struct {
	Key string `json:"key"`
	OK  bool   `json:"ok"`
	Err string `json:"error,omitempty"`
}

// Batch 逐键执行 delete/visibility/move，部分成功。复用单键方法保证归属与校验一致。
func (s *Service) Batch(userID uint64, action string, keys []string, visibility string, albumID *int64) ([]BatchResult, error) {
	switch action {
	case "delete", "visibility", "move":
	default:
		return nil, ErrInvalidAction
	}
	out := make([]BatchResult, 0, len(keys))
	for _, k := range keys {
		var err error
		switch action {
		case "delete":
			err = s.SoftDelete(userID, k)
		case "visibility":
			v := visibility
			_, err = s.Update(userID, k, nil, &v, nil, nil, false, nil)
		case "move":
			_, err = s.Update(userID, k, nil, nil, albumID, nil, false, nil)
		}
		br := BatchResult{Key: k, OK: err == nil}
		if err != nil {
			br.Err = err.Error()
		}
		out = append(out, br)
	}
	return out, nil
}

// TrashList 列软删项（deleted_at 倒序，简单 offset-free：按 deleted_at,id keyset 略——回收站量小，用 id 游标）。
func (s *Service) TrashList(userID uint64, cursor string, limit int) ([]Row, string, error) {
	if limit <= 0 {
		limit = 24
	}
	q := s.db.Unscoped().Table("images").
		Joins("JOIN files ON files.id = images.file_id").
		Where("images.user_id = ? AND images.deleted_at IS NOT NULL", userID)
	if cursor != "" {
		cur, err := decodeListCursor(cursor)
		if err != nil || cur.Sort != "trash" {
			return nil, "", ErrBadCursor
		}
		q = q.Where("images.id < ?", cur.ID)
	}
	q = q.Select("images.*, files.size AS sort_size").
		Order("images.id DESC").Limit(limit + 1)
	var scans []listScan
	if err := q.Scan(&scans).Error; err != nil {
		return nil, "", err
	}
	next := ""
	if len(scans) > limit {
		next = encodeListCursor(listCursor{Sort: "trash", ID: scans[limit-1].ID})
		scans = scans[:limit]
	}
	rows, err := s.hydrate(scans)
	if err != nil {
		return nil, "", err
	}
	return rows, next, nil
}

// Restore 恢复软删项。非属主/未软删→ErrNotFound。
func (s *Service) Restore(userID uint64, key string) error {
	res := s.db.Unscoped().Model(&model.Image{}).
		Where("key = ? AND user_id = ? AND deleted_at IS NOT NULL", key, userID).
		Update("deleted_at", nil)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// purgeOne 在事务内彻底删除一张(已软删或含软删)image：退还属主配额、file.ref_count-1、硬删 image。
// 返回该操作是否使某 file 的 ref_count 归零及其 (policyID, path, thumbKey)，供事务外投递物理删除。
type physicalDelete struct {
	policyID uint64
	path     string
	// thumbs 当前世代 + 遗留缩略键,Delete 幂等(C2 ThumbGen dual-probe)。
	thumbs []string
}

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

func (s *Service) purgeOne(tx *gorm.DB, img *model.Image) (*physicalDelete, error) {
	// 幂等门禁：先硬删 image 行(仍在回收站者)，只有真正删掉(RowsAffected==1)才继续退配额/减引用。
	// 防并发/重试双删导致双退配额，以及误删仍被其它 live 图引用的共享物理文件。
	res := tx.Unscoped().Delete(&model.Image{}, "id = ? AND deleted_at IS NOT NULL", img.ID)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, nil // 已被并发/上一轮清理(或已恢复)，幂等 no-op
	}
	var file model.File
	if err := tx.First(&file, img.FileID).Error; err != nil {
		return nil, err
	}
	// 退还配额（软删未退，此刻退）——仅当有属主；CASE 夹到 ≥0，防陈旧计数双退变负
	if img.UserID != nil {
		if err := tx.Model(&model.User{}).Where("id = ?", *img.UserID).
			UpdateColumn("used_storage", gorm.Expr(
				"CASE WHEN used_storage >= ? THEN used_storage - ? ELSE 0 END",
				file.Size, file.Size,
			)).Error; err != nil {
			return nil, err
		}
	}
	// ref_count-1（原子）
	if err := tx.Model(&model.File{}).Where("id = ?", file.ID).
		UpdateColumn("ref_count", gorm.Expr("ref_count - ?", 1)).Error; err != nil {
		return nil, err
	}
	// 条件删：仅当减后 ≤0 才删行；并发秒传 +1 后本条 0 行，file 保留（purge-vs-秒传）
	del := tx.Where("id = ? AND ref_count <= 0", file.ID).Delete(&model.File{})
	if del.Error != nil {
		return nil, del.Error
	}
	if del.RowsAffected == 1 {
		return &physicalDelete{
			policyID: file.StoragePolicyID, path: file.Path,
			thumbs: storagesvc.ThumbKeyCandidates(file.Surface, file.Hash),
		}, nil
	}
	return nil, nil
}

func (s *Service) enqueuePhysical(pd *physicalDelete) {
	if pd == nil || s.run == nil {
		return
	}
	keys := append([]string{pd.path}, pd.thumbs...)
	for _, key := range keys {
		if key == "" {
			continue
		}
		payload, _ := json.Marshal(map[string]any{"policy_id": pd.policyID, "key": key})
		if err := s.run.Enqueue("delete_file", string(payload)); err != nil {
			slog.Error("投递物理删除失败", "policy_id", pd.policyID, "key", key, "err", err)
		}
	}
}

// DeleteUserData 注销级联:单事务硬删该用户全部图片(先把 live 图软删,复用
// purgeOne 的幂等门禁与引用计数)、相册、API token、auth token、session、用户行。
// 物理文件删除在事务提交后投递(归零 file 才删,秒传共享文件安全)。
// 不依赖 DB 级 CASCADE——存量 SQLite 库无外键约束(⑤a 裁决),关联表显式删。
func (s *Service) DeleteUserData(userID uint64) error {
	var pds []*physicalDelete
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Model(&model.Image{}).
			Where("user_id = ? AND deleted_at IS NULL", userID).
			Update("deleted_at", time.Now()).Error; err != nil {
			return err
		}
		var imgs []model.Image
		if err := tx.Unscoped().Where("user_id = ?", userID).Find(&imgs).Error; err != nil {
			return err
		}
		for i := range imgs {
			pd, err := s.purgeOne(tx, &imgs[i])
			if err != nil {
				return err
			}
			if pd != nil {
				pds = append(pds, pd)
			}
		}
		if err := tx.Delete(&model.Album{}, "user_id = ?", userID).Error; err != nil {
			return err
		}
		if err := tx.Delete(&model.APIToken{}, "user_id = ?", userID).Error; err != nil {
			return err
		}
		if err := tx.Delete(&model.AuthToken{}, "user_id = ?", userID).Error; err != nil {
			return err
		}
		if err := tx.Delete(&model.Session{}, "user_id = ?", userID).Error; err != nil {
			return err
		}
		return tx.Delete(&model.User{}, userID).Error
	})
	if err != nil {
		return err
	}
	for _, pd := range pds {
		s.enqueuePhysical(pd)
	}
	return nil
}

// PurgePermanent 彻底删除属主的一张软删图。非属主/不在回收站→ErrNotFound。
func (s *Service) PurgePermanent(userID uint64, key string) error {
	var img model.Image
	err := s.db.Unscoped().
		Where("key = ? AND user_id = ? AND deleted_at IS NOT NULL", key, userID).
		First(&img).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	var pd *physicalDelete
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		var e error
		pd, e = s.purgeOne(tx, &img)
		return e
	}); err != nil {
		return err
	}
	s.enqueuePhysical(pd)
	return nil
}

// EmptyTrash 彻底删除属主回收站全部，返回清理张数。
func (s *Service) EmptyTrash(userID uint64) (int, error) {
	var imgs []model.Image
	if err := s.db.Unscoped().
		Where("user_id = ? AND deleted_at IS NOT NULL", userID).Find(&imgs).Error; err != nil {
		return 0, err
	}
	var pds []*physicalDelete
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		for i := range imgs {
			pd, e := s.purgeOne(tx, &imgs[i])
			if e != nil {
				return e
			}
			pds = append(pds, pd)
		}
		return nil
	}); err != nil {
		return 0, err
	}
	for _, pd := range pds {
		s.enqueuePhysical(pd)
	}
	return len(imgs), nil
}

// PurgeExpiredTrash 全站清理 deleted_at 超过 30 天的软删图（server 定时调用）。
func (s *Service) PurgeExpiredTrash(ctx context.Context) (int, error) {
	cutoff := time.Now().Add(-30 * 24 * time.Hour)
	var imgs []model.Image
	if err := s.db.Unscoped().
		Where("deleted_at IS NOT NULL AND deleted_at < ?", cutoff).Find(&imgs).Error; err != nil {
		return 0, err
	}
	n := 0
	for i := range imgs {
		var pd *physicalDelete
		if err := s.db.Transaction(func(tx *gorm.DB) error {
			var e error
			pd, e = s.purgeOne(tx, &imgs[i])
			return e
		}); err != nil {
			slog.Error("清理过期软删失败", "image_id", imgs[i].ID, "err", err)
			continue
		}
		s.enqueuePhysical(pd)
		n++
	}
	return n, nil
}

// PurgeExpiredImages 物理清理到期(expires_at<now)的 live 图。过期即永久删除、回收配额,
// 不进回收站(否则延迟 30 天回收违背回收存储初衷)。server 每小时调用。
func (s *Service) PurgeExpiredImages(ctx context.Context) (int, error) {
	var imgs []model.Image
	if err := s.db.Where("expires_at IS NOT NULL AND expires_at < ? AND deleted_at IS NULL", time.Now()).
		Find(&imgs).Error; err != nil {
		return 0, err
	}
	n := 0
	for i := range imgs {
		var pd *physicalDelete
		did := false
		if err := s.db.Transaction(func(tx *gorm.DB) error {
			// 事务内条件软删:重新确认 DB 中仍过期(expires_at<now)、未被并发清除/延长/删除,
			// 防按 Find 陈旧快照永久误删已改期的图(codex 评审竞态)。RowsAffected==0 即跳过。
			// 软删置 deleted_at 以过 purgeOne 的 deleted_at NOT NULL 门禁,再复用其配额/引用/
			// 文件物理删逻辑——过期图直接永久清理,不经回收站。
			res := tx.Where("id = ? AND expires_at IS NOT NULL AND expires_at < ? AND deleted_at IS NULL",
				imgs[i].ID, time.Now()).Delete(&model.Image{})
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				return nil // 竞态:已改期/已删/已清,不 purge
			}
			did = true
			var e error
			pd, e = s.purgeOne(tx, &imgs[i])
			return e
		}); err != nil {
			slog.Error("清理过期图失败", "image_id", imgs[i].ID, "err", err)
			continue
		}
		if !did {
			continue
		}
		s.enqueuePhysical(pd)
		n++
	}
	return n, nil
}
