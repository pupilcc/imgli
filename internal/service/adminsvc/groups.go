package adminsvc

import (
	"errors"
	"strings"

	"gorm.io/gorm"

	"github.com/yixian-huang/imgli/internal/model"
)

var (
	// ErrExtsEmpty allowed_exts 必须是非空数组（每项非空，写入前统一小写化）。
	ErrExtsEmpty = errors.New("allowed_exts 不能为空")
	// ErrGroupNameInvalid 组名需 1-64 个字符（TrimSpace 后）。
	ErrGroupNameInvalid = errors.New("组名需 1-64 个字符")
	// ErrQuotaInvalid storage_quota / max_file_size 必须 > 0。
	ErrQuotaInvalid = errors.New("storage_quota 与 max_file_size 需 > 0")
	// ErrBandwidthQuotaInvalid bandwidth_quota_month 必须 >= 0（0=不限制）。
	ErrBandwidthQuotaInvalid = errors.New("bandwidth_quota_month 需 >= 0")
	// ErrPolicyNotFound allowed_policy_ids 中存在未知的存储策略 id。
	ErrPolicyNotFound = errors.New("存储策略不存在")
	// ErrBuiltinGroup 内置组（IsDefault/IsGuest）不可改名或删除——防止误改语义锚点。
	// 配额/限速/允许后缀/允许策略等其余字段仍可正常修改。
	ErrBuiltinGroup = errors.New("内置组不可改名或删除")
	// ErrGroupInUse 组内仍有用户时不可删除。
	ErrGroupInUse = errors.New("组内存在用户，不可删除")
)

// GroupRow 是列表项：用户组 + 实时算出的组内用户数。
type GroupRow struct {
	Group     model.UserGroup
	UserCount int64
}

// GroupPatch 是 UpdateGroup 的部分更新载荷：nil 字段保持不变。
type GroupPatch struct {
	Name                *string
	StorageQuota        *int64
	MaxFileSize         *int64
	BandwidthQuotaMonth *int64 // >=0；0=不限制
	RatePerMinute       *int
	RatePerHour         *int
	RatePerDay          *int
	AllowedExts         *[]string
	AllowedPolicyIDs    *[]uint64
}

// ListGroups 按 id 升序返回全部用户组，含每组实时用户数。组数量少，不分页。
func (s *Service) ListGroups() ([]GroupRow, error) {
	var groups []model.UserGroup
	if err := s.db.Order("id").Find(&groups).Error; err != nil {
		return nil, err
	}
	rows := make([]GroupRow, 0, len(groups))
	for i := range groups {
		var n int64
		if err := s.db.Model(&model.User{}).Where("group_id = ?", groups[i].ID).Count(&n).Error; err != nil {
			return nil, err
		}
		rows = append(rows, GroupRow{Group: groups[i], UserCount: n})
	}
	return rows, nil
}

// GroupByID 按 id 取单个用户组（不存在 → ErrGroupNotFound）；供 handler 侧写操作前后取展示字段（如 audit 里的 name）。
func (s *Service) GroupByID(id uint64) (*model.UserGroup, error) {
	var g model.UserGroup
	if err := s.db.First(&g, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGroupNotFound
		}
		return nil, err
	}
	return &g, nil
}

// normalizeExts 校验 allowed_exts 为非空数组，逐项 TrimSpace+ToLower 后仍非空，并去重
// （保留首次出现的顺序）；返回规整后的切片。
func normalizeExts(exts []string) ([]string, error) {
	if len(exts) == 0 {
		return nil, ErrExtsEmpty
	}
	seen := make(map[string]bool, len(exts))
	norm := make([]string, 0, len(exts))
	for _, e := range exts {
		e = strings.ToLower(strings.TrimSpace(e))
		if e == "" {
			return nil, ErrExtsEmpty
		}
		if seen[e] {
			continue
		}
		seen[e] = true
		norm = append(norm, e)
	}
	return norm, nil
}

// policyIDsExist 校验 ids 中每个存储策略都存在；ids 为空视为"不限"直接放行——
// 上传层已有的 AllowedPolicyIDs 为空时回退可用策略的逻辑决定实际语义，本层不强制非空。
func (s *Service) policyIDsExist(ids []uint64) error {
	if len(ids) == 0 {
		return nil
	}
	var existing []uint64
	if err := s.db.Model(&model.StoragePolicy{}).Where("id IN ?", ids).Pluck("id", &existing).Error; err != nil {
		return err
	}
	exist := make(map[uint64]bool, len(existing))
	for _, id := range existing {
		exist[id] = true
	}
	for _, id := range ids {
		if !exist[id] {
			return ErrPolicyNotFound
		}
	}
	return nil
}

// CreateGroup 创建用户组。校验：Name 非空且 ≤64；StorageQuota/MaxFileSize > 0；
// AllowedExts 非空数组（写入前小写化）；AllowedPolicyIDs 中的策略必须存在（空数组允许）。
// IsDefault/IsGuest 内置标志不可经此途径设置，创建后强制为 false。
func (s *Service) CreateGroup(g *model.UserGroup) error {
	name := strings.TrimSpace(g.Name)
	if name == "" || len(name) > 64 {
		return ErrGroupNameInvalid
	}
	if g.StorageQuota <= 0 || g.MaxFileSize <= 0 {
		return ErrQuotaInvalid
	}
	if g.BandwidthQuotaMonth < 0 {
		return ErrBandwidthQuotaInvalid
	}
	exts, err := normalizeExts(g.AllowedExts)
	if err != nil {
		return err
	}
	if err := s.policyIDsExist(g.AllowedPolicyIDs); err != nil {
		return err
	}
	g.Name = name
	g.AllowedExts = exts
	g.IsDefault = false
	g.IsGuest = false
	return s.db.Create(g).Error
}

// UpdateGroup 部分更新用户组。内置组（IsDefault/IsGuest）可改配额/限速/允许后缀/允许策略，
// 但不可改名（ErrBuiltinGroup）。
//
// 写回只 Select 本次实际 patch 到的列（而非整行 Save）——若整行覆盖，First 与写入之间
// 若有另一个并发 PATCH 先提交了别的字段，本次写回会用 First 时读到的陈旧快照把那些字段
// 覆盖回旧值（lost update）。只 Select patch 到的列，未涉及字段完全不出现在 UPDATE
// 语句里，天然避免这个问题。走结构体路径（而非 map Updates）：AllowedExts/AllowedPolicyIDs
// 带 serializer:json，map 路径的 clause.Assignment 不经过字段序列化器，直接把
// []string/[]uint64 递给 SQL 驱动会出错，只有结构体路径的 field.ValueOf 才会正确序列化。
// 顺带用 RowsAffected 门禁：First 与 Updates 之间若该组被并发删除，报 ErrGroupNotFound
// 而非用陈旧内存对象冒充更新成功。
func (s *Service) UpdateGroup(id uint64, p GroupPatch) (*model.UserGroup, error) {
	var g model.UserGroup
	if err := s.db.First(&g, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGroupNotFound
		}
		return nil, err
	}
	builtin := g.IsDefault || g.IsGuest

	var cols []string
	if p.Name != nil {
		if builtin {
			return nil, ErrBuiltinGroup
		}
		name := strings.TrimSpace(*p.Name)
		if name == "" || len(name) > 64 {
			return nil, ErrGroupNameInvalid
		}
		g.Name = name
		cols = append(cols, "name")
	}
	if p.StorageQuota != nil {
		if *p.StorageQuota <= 0 {
			return nil, ErrQuotaInvalid
		}
		g.StorageQuota = *p.StorageQuota
		cols = append(cols, "storage_quota")
	}
	if p.MaxFileSize != nil {
		if *p.MaxFileSize <= 0 {
			return nil, ErrQuotaInvalid
		}
		g.MaxFileSize = *p.MaxFileSize
		cols = append(cols, "max_file_size")
	}
	if p.BandwidthQuotaMonth != nil {
		if *p.BandwidthQuotaMonth < 0 {
			return nil, ErrBandwidthQuotaInvalid
		}
		g.BandwidthQuotaMonth = *p.BandwidthQuotaMonth
		cols = append(cols, "bandwidth_quota_month")
	}
	if p.RatePerMinute != nil {
		g.RatePerMinute = *p.RatePerMinute
		cols = append(cols, "rate_per_minute")
	}
	if p.RatePerHour != nil {
		g.RatePerHour = *p.RatePerHour
		cols = append(cols, "rate_per_hour")
	}
	if p.RatePerDay != nil {
		g.RatePerDay = *p.RatePerDay
		cols = append(cols, "rate_per_day")
	}
	if p.AllowedExts != nil {
		exts, err := normalizeExts(*p.AllowedExts)
		if err != nil {
			return nil, err
		}
		g.AllowedExts = exts
		cols = append(cols, "allowed_exts")
	}
	if p.AllowedPolicyIDs != nil {
		ids := *p.AllowedPolicyIDs
		if err := s.policyIDsExist(ids); err != nil {
			return nil, err
		}
		g.AllowedPolicyIDs = ids
		cols = append(cols, "allowed_policy_ids")
	}

	if len(cols) == 0 {
		return &g, nil
	}

	res := s.db.Model(&g).Select(cols).Updates(&g)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, ErrGroupNotFound
	}
	return &g, nil
}

// DeleteGroup 删除用户组。内置组不可删（ErrBuiltinGroup）；组内仍有用户不可删（ErrGroupInUse）；不存在 → ErrGroupNotFound。
// 末尾写步显式 RowsAffected 门禁（同 images.go AdminSoftDelete 的既有模式）：First 与
// Delete 之间若该组被并发删除，Delete 匹配不到行而 RowsAffected==0，此时必须报
// ErrGroupNotFound，不得让前面 First/校验都通过就冒充删除成功。
func (s *Service) DeleteGroup(id uint64) error {
	var g model.UserGroup
	if err := s.db.First(&g, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrGroupNotFound
		}
		return err
	}
	if g.IsDefault || g.IsGuest {
		return ErrBuiltinGroup
	}
	var n int64
	if err := s.db.Model(&model.User{}).Where("group_id = ?", id).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return ErrGroupInUse
	}
	res := s.db.Delete(&model.UserGroup{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrGroupNotFound
	}
	return nil
}
