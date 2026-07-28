package adminsvc

import (
	"crypto/rand"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/yixian-huang/imgli/internal/model"
)

var (
	ErrInviteCountInvalid = errors.New("count 需为 1-100")
	ErrInviteNotFound     = errors.New("邀请码不存在")
	ErrInviteUsed         = errors.New("已使用的邀请码不可撤销")
)

// inviteCharset 剔除 0O1I 易混字符。
const inviteCharset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// newInviteCode 生成 IL-XXXX-XXXX。
func newInviteCode() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	out := []byte("IL-XXXX-XXXX")
	pos := []int{3, 4, 5, 6, 8, 9, 10, 11}
	for i, p := range pos {
		out[p] = inviteCharset[int(b[i])%len(inviteCharset)]
	}
	return string(out), nil
}

// InviteRow 列表项:码 + 派生 status + 创建者/使用者用户名(用户已删则空)。
type InviteRow struct {
	Invite        model.InviteCode
	Status        string
	CreatedByName string
	UsedByName    string
}

// CreateInvites 批量生成邀请码(count 1-100);expiresInDays<=0 表示不过期。
// 返回明文码列表。碰撞由 uniqueIndex 兜底,单码最多重试 5 次。
func (s *Service) CreateInvites(createdBy uint64, count, expiresInDays int) ([]string, error) {
	if count < 1 || count > 100 {
		return nil, ErrInviteCountInvalid
	}
	var expires *time.Time
	if expiresInDays > 0 {
		t := time.Now().AddDate(0, 0, expiresInDays)
		expires = &t
	}
	codes := make([]string, 0, count)
	for len(codes) < count {
		var lastErr error
		for attempt := 0; attempt < 5; attempt++ {
			code, err := newInviteCode()
			if err != nil {
				return nil, err
			}
			err = s.db.Create(&model.InviteCode{Code: code, CreatedBy: createdBy, ExpiresAt: expires}).Error
			if err == nil {
				codes = append(codes, code)
				lastErr = nil
				break
			}
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				lastErr = err
				continue
			}
			return nil, err
		}
		if lastErr != nil {
			return nil, lastErr
		}
	}
	return codes, nil
}

// ListInvites 按状态筛选分页。status: unused|used|expired|""(全部)。
// WHERE 筛选与逐行 Status 派生共用同一 now，避免临界到期码筛/标不一致。
func (s *Service) ListInvites(status string, page, limit int) ([]InviteRow, int64, error) {
	now := time.Now()
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = defaultListLimit
	}
	if limit > maxListLimit {
		limit = maxListLimit
	}
	tx := s.db.Model(&model.InviteCode{})
	switch status {
	case "unused":
		tx = tx.Where("used_by IS NULL AND (expires_at IS NULL OR expires_at > ?)", now)
	case "used":
		tx = tx.Where("used_by IS NOT NULL")
	case "expired":
		tx = tx.Where("used_by IS NULL AND expires_at IS NOT NULL AND expires_at <= ?", now)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var ics []model.InviteCode
	if err := tx.Order("id DESC").Offset((page - 1) * limit).Limit(limit).Find(&ics).Error; err != nil {
		return nil, 0, err
	}
	// 批量取用户名(创建者+使用者),一次查询
	idSet := map[uint64]struct{}{}
	for _, ic := range ics {
		idSet[ic.CreatedBy] = struct{}{}
		if ic.UsedBy != nil {
			idSet[*ic.UsedBy] = struct{}{}
		}
	}
	names := map[uint64]string{}
	if len(idSet) > 0 {
		ids := make([]uint64, 0, len(idSet))
		for id := range idSet {
			ids = append(ids, id)
		}
		var users []model.User
		if err := s.db.Select("id, username").Where("id IN ?", ids).Find(&users).Error; err != nil {
			return nil, 0, err
		}
		for _, u := range users {
			names[u.ID] = u.Username
		}
	}
	rows := make([]InviteRow, len(ics))
	for i, ic := range ics {
		st := "unused"
		switch {
		case ic.UsedBy != nil:
			st = "used"
		case ic.ExpiresAt != nil && !ic.ExpiresAt.After(now):
			st = "expired"
		}
		rows[i] = InviteRow{
			Invite: ic, Status: st, CreatedByName: names[ic.CreatedBy],
		}
		if ic.UsedBy != nil {
			rows[i].UsedByName = names[*ic.UsedBy]
		}
	}
	return rows, total, nil
}

// RevokeInvite 删除未被使用的码(含已过期——这是过期码的清理路径);已用码拒绝(历史留档)。
// 成功时返回被删码明文 Code，错误路径返回 ("", err)。
func (s *Service) RevokeInvite(id uint64) (string, error) {
	var ic model.InviteCode
	if err := s.db.First(&ic, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrInviteNotFound
		}
		return "", err
	}
	if ic.UsedBy != nil {
		return "", ErrInviteUsed
	}
	// 条件删除防与"正被核销"竞态:核销后 used_by 非空,此删除 0 行
	res := s.db.Where("id = ? AND used_by IS NULL", id).Delete(&model.InviteCode{})
	if res.Error != nil {
		return "", res.Error
	}
	if res.RowsAffected == 0 {
		return "", ErrInviteUsed
	}
	return ic.Code, nil
}
