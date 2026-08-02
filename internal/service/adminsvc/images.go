package adminsvc

import (
	"errors"

	"gorm.io/gorm"

	"github.com/yixian-huang/imgli/internal/model"
)

// ErrImageNotFound 目标图片不存在（含已软删——管理面幂等裁决：重复操作按 404 处理即可）。
var ErrImageNotFound = errors.New("图片不存在")

// ImageRow 是全站图片列表一条：图片连同物理文件、存储策略与属主用户名。
// 游客图（Img.UserID 为 nil）Username 为空字符串。
type ImageRow struct {
	Img      model.Image
	File     model.File
	Policy   model.StoragePolicy
	Username string
}

// imageScan 承接 JOIN 查询扫描：images.* 加 LEFT JOIN users 出的 username。
type imageScan struct {
	model.Image
	Username string `gorm:"column:username"`
}

// imagesBaseQuery 构造全站图片查询的公共 JOIN/WHERE（LEFT JOIN users 容纳游客图）。
// deleted: ""|"live"=仅未软删（默认）；"trash"=仅回收站；"all"=含软删。
// 不含 Select/Order/Limit，供 Count 与列表分别追加。
func (s *Service) imagesBaseQuery(userID uint64, status string, policyID uint64, deleted string) *gorm.DB {
	q := s.db.Table("images").
		Joins("JOIN files ON files.id = images.file_id").
		Joins("LEFT JOIN users ON users.id = images.user_id")
	switch deleted {
	case "trash":
		q = q.Where("images.deleted_at IS NOT NULL")
	case "all":
		// 不筛 deleted_at
	default: // live / ""
		q = q.Where("images.deleted_at IS NULL")
	}
	if userID > 0 {
		q = q.Where("images.user_id = ?", userID)
	}
	if status != "" {
		q = q.Where("images.status = ?", status)
	}
	if policyID > 0 {
		q = q.Where("files.storage_policy_id = ?", policyID)
	}
	return q
}

// ListImages 全站图片列表（不限属主，含游客图）。status 空=不筛，否则 ∈ normal|pending|rejected
// （非法取值由 handler 层 400 校验）；policyID>0 按文件所属存储策略筛；
// deleted ∈ ""|live|trash|all。按 images.id 倒序。
func (s *Service) ListImages(userID uint64, status string, policyID uint64, deleted string, page, limit int) ([]ImageRow, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = defaultListLimit
	}
	if limit > maxListLimit {
		limit = maxListLimit
	}
	var total int64
	if err := s.imagesBaseQuery(userID, status, policyID, deleted).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var scans []imageScan
	err := s.imagesBaseQuery(userID, status, policyID, deleted).
		Select("images.*, users.username AS username").
		Order("images.id DESC").
		Offset((page - 1) * limit).Limit(limit).
		Scan(&scans).Error
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.hydrateImageRows(scans)
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// GetImageRow 取单图连同 File/Policy/属主用户名（供 handler 在写操作后构造含 links 的响应）。
// 排除软删；不存在→ErrImageNotFound。
func (s *Service) GetImageRow(key string) (*ImageRow, error) {
	var scans []imageScan
	err := s.db.Table("images").
		Joins("JOIN files ON files.id = images.file_id").
		Joins("LEFT JOIN users ON users.id = images.user_id").
		Where("images.key = ? AND images.deleted_at IS NULL", key).
		Select("images.*, users.username AS username").
		Limit(1).
		Scan(&scans).Error
	if err != nil {
		return nil, err
	}
	if len(scans) == 0 {
		return nil, ErrImageNotFound
	}
	rows, err := s.hydrateImageRows(scans)
	if err != nil {
		return nil, err
	}
	return &rows[0], nil
}

// hydrateImageRows 批量装载 File 与 StoragePolicy（避免 N+1），拼上已随扫描带出的 Username。
func (s *Service) hydrateImageRows(scans []imageScan) ([]ImageRow, error) {
	if len(scans) == 0 {
		return []ImageRow{}, nil
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
	out := make([]ImageRow, 0, len(scans))
	for i := range scans {
		f := fileByID[scans[i].FileID]
		out = append(out, ImageRow{
			Img: scans[i].Image, File: f, Policy: polByID[f.StoragePolicyID],
			Username: scans[i].Username,
		})
	}
	return out, nil
}

// AdminSoftDelete 管理员软删任意用户的图（进属主回收站，直链转 410）。
// 单语句软删 + RowsAffected 门禁（同 imagesvc.SoftDelete 既有写法）：找不到或已软删
// （默认 scope 天然把已软删行排除在 WHERE 之外，故此时 RowsAffected==0）→ ErrImageNotFound，
// 不依赖先 First 再写的两步——避免并发双删/删改竞态下输者 0 行更新却误报成功。
func (s *Service) AdminSoftDelete(key string) (*model.Image, error) {
	res := s.db.Where("key = ?", key).Delete(&model.Image{})
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, ErrImageNotFound
	}
	// 已确认软删成功；Unscoped 取回该(已软删)行供 audit 用 owner_id 等字段。
	var img model.Image
	if err := s.db.Unscoped().Where("key = ?", key).First(&img).Error; err != nil {
		return nil, err
	}
	return &img, nil
}

// SetWhitelist 写 is_whitelisted；on=true 且当前 status≠normal 时一并复位 status=normal（裁决 4）。
// 写步显式 RowsAffected 门禁：First 与 Update 之间若该图被并发软删，Update 因
// "deleted_at IS NULL" 匹配不到行而 RowsAffected==0，此时必须报 ErrImageNotFound，
// 不得用 First 阶段读到的陈旧 img 冒充成功（同 AdminSoftDelete 的防假成功原则）。
// 不存在→ErrImageNotFound。
func (s *Service) SetWhitelist(key string, on bool) (*model.Image, error) {
	var img model.Image
	if err := s.db.Where("key = ?", key).First(&img).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrImageNotFound
		}
		return nil, err
	}
	updates := map[string]any{"is_whitelisted": on}
	img.IsWhitelisted = on
	if on && img.Status != "normal" {
		updates["status"] = "normal"
		img.Status = "normal"
	}
	res := s.db.Model(&model.Image{}).Where("id = ? AND deleted_at IS NULL", img.ID).Updates(updates)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, ErrImageNotFound
	}
	return &img, nil
}
