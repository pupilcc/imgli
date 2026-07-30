package imagesvc

import (
	"errors"
	"fmt"
	"time"

	"github.com/yixian-huang/imgli/internal/model"
)

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

func normSort(sort string) string {
	if sort == "" {
		return "date"
	}
	return sort
}

// hydrate 批量装载 File 与 StoragePolicy，避免 N+1。

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

// MaxViewsMax 单图允许的最大访问次数上限（防止滥用）。
const MaxViewsMax = 10000

// ErrInvalidMaxViews max_views 不合法。
var ErrInvalidMaxViews = errors.New("imagesvc: max_views 须为 0 或 1–10000")

// ErrInvalidAccessPassword 访问口令不合法。
var ErrInvalidAccessPassword = errors.New("imagesvc: access_password 过长或不合法")

// Update 部分更新单图（见 UpdatePatch）。
