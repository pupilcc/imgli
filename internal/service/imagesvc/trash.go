package imagesvc

import (
	"github.com/yixian-huang/imgli/internal/model"
)

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
// copyObjectKey 把 src 键对象复制到 dst 键。读入内存再 Put:s3 Put 需 Content-Length
// (bodyLen 认 bytes.Reader.Len()),而 driver.Open 返回的 reader(s3 rangeReadSeekCloser)
// 无 Len()→chunked→S3 拒 MissingContentLength。切换罕见、对象 MB 级,缓冲可接受。
