package adminsvc

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/auth"
)

var (
	ErrUserNotFound  = errors.New("用户不存在")
	ErrGroupNotFound = errors.New("用户组不存在")
	ErrSelfBan       = errors.New("不能封禁自己")
	ErrInvalidStatus = errors.New("status 仅支持 active|banned")
)

// UserRow 是列表项：用户 + 实时算出的图片计数 + 最近会话时间（sessions 最大 created_at）。
type UserRow struct {
	User       model.User
	ImageCount int64
	// LastSeenAt 最近一次签发 Web session 的时间；无 session 为 nil（非持续心跳）。
	LastSeenAt *time.Time
}

const (
	defaultListLimit = 50
	maxListLimit     = 200
)

// ListUsers 按 q（username/email LIKE）、group_id、status、signup channel 筛选。
// sort：id（默认升序）| bandwidth | storage | created | last_seen。
func (s *Service) ListUsers(q string, groupID uint64, status, channel, sort string, page, limit int) ([]UserRow, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = defaultListLimit
	}
	if limit > maxListLimit {
		limit = maxListLimit
	}
	tx := s.db.Model(&model.User{})
	if q != "" {
		like := "%" + q + "%"
		tx = tx.Where("username LIKE ? OR email LIKE ?", like, like)
	}
	if groupID > 0 {
		tx = tx.Where("group_id = ?", groupID)
	}
	if status != "" {
		tx = tx.Where("status = ?", status)
	}
	if channel != "" {
		tx = tx.Where("signup_channel = ?", channel)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	order := usersOrder(sort)
	var users []model.User
	if err := tx.Order(order).Offset((page - 1) * limit).Limit(limit).Find(&users).Error; err != nil {
		return nil, 0, err
	}
	counts, err := s.imageCountsByUser(users)
	if err != nil {
		return nil, 0, err
	}
	seen, err := s.lastSeenByUser(users)
	if err != nil {
		return nil, 0, err
	}
	rows := make([]UserRow, 0, len(users))
	for i := range users {
		rows = append(rows, UserRow{
			User: users[i], ImageCount: counts[users[i].ID], LastSeenAt: seen[users[i].ID],
		})
	}
	return rows, total, nil
}

// usersOrder 白名单排序，未知值回落 id。
func usersOrder(sort string) string {
	switch sort {
	case "bandwidth":
		return "bandwidth_used_month DESC, id DESC"
	case "storage":
		return "used_storage DESC, id DESC"
	case "created":
		return "created_at DESC, id DESC"
	case "last_seen":
		// 无 session 的用户排后；子查询兼容 SQLite/Postgres。
		return "(SELECT MAX(created_at) FROM sessions WHERE sessions.user_id = users.id) DESC, id DESC"
	default:
		return "id ASC"
	}
}

// imageCountsByUser 一次 GROUP BY 取本页用户的图片数（软删不计）；无用户返回空 map。
func (s *Service) imageCountsByUser(users []model.User) (map[uint64]int64, error) {
	out := make(map[uint64]int64, len(users))
	if len(users) == 0 {
		return out, nil
	}
	ids := make([]uint64, len(users))
	for i := range users {
		ids[i] = users[i].ID
	}
	var agg []struct {
		UserID uint64
		N      int64
	}
	err := s.db.Model(&model.Image{}).
		Select("user_id, count(*) as n").
		Where("user_id IN ?", ids).
		Group("user_id").
		Scan(&agg).Error
	if err != nil {
		return nil, err
	}
	for _, a := range agg {
		out[a.UserID] = a.N
	}
	return out, nil
}

// lastSeenByUser 本页用户最近一次 Web session 签发时间（sessions.created_at MAX）。
// SQLite 的 MAX(datetime) 常以字符串扫出，故用 sql 可空字符串再 Parse。
func (s *Service) lastSeenByUser(users []model.User) (map[uint64]*time.Time, error) {
	out := make(map[uint64]*time.Time, len(users))
	if len(users) == 0 {
		return out, nil
	}
	ids := make([]uint64, len(users))
	for i := range users {
		ids[i] = users[i].ID
	}
	var agg []struct {
		UserID uint64
		T      *string
	}
	err := s.db.Model(&model.Session{}).
		Select("user_id, MAX(created_at) AS t").
		Where("user_id IN ?", ids).
		Group("user_id").
		Scan(&agg).Error
	if err != nil {
		return nil, err
	}
	for _, a := range agg {
		if a.T == nil || *a.T == "" {
			continue
		}
		// GORM/SQLite 常见 "2006-01-02 15:04:05.999999999-07:00" 或 RFC3339
		if ts, e := time.Parse(time.RFC3339Nano, *a.T); e == nil {
			out[a.UserID] = &ts
			continue
		}
		if ts, e := time.Parse(time.RFC3339, *a.T); e == nil {
			out[a.UserID] = &ts
			continue
		}
		if ts, e := time.ParseInLocation("2006-01-02 15:04:05.999999999-07:00", *a.T, time.Local); e == nil {
			out[a.UserID] = &ts
			continue
		}
		if ts, e := time.ParseInLocation("2006-01-02 15:04:05", *a.T, time.Local); e == nil {
			out[a.UserID] = &ts
			continue
		}
	}
	return out, nil
}

// UpdateUser 改组/改状态。禁止对自己置 banned；校验 status 取值、目标用户与目标组存在。
func (s *Service) UpdateUser(actorID, targetID uint64, groupID *uint64, status *string) (*model.User, error) {
	if status != nil {
		if *status != "active" && *status != "banned" {
			return nil, ErrInvalidStatus
		}
		if targetID == actorID && *status == "banned" {
			return nil, ErrSelfBan
		}
	}
	var u model.User
	if err := s.db.First(&u, targetID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	if groupID != nil {
		var g model.UserGroup
		if err := s.db.First(&g, *groupID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, ErrGroupNotFound
			}
			return nil, err
		}
	}
	updates := map[string]any{}
	if groupID != nil {
		updates["group_id"] = *groupID
		u.GroupID = *groupID
	}
	if status != nil {
		updates["status"] = *status
		u.Status = *status
	}
	if len(updates) == 0 {
		return &u, nil
	}
	if err := s.db.Model(&model.User{}).Where("id = ?", u.ID).Updates(updates).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

const resetPasswordLen = 16

// ResetPassword 生成 16 字符一次性明文密码，argon2id 入库，并删除该用户全部 session（强制登出）。
func (s *Service) ResetPassword(targetID uint64) (string, error) {
	var u model.User
	if err := s.db.First(&u, targetID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrUserNotFound
		}
		return "", err
	}
	raw := make([]byte, 12) // base64url 无填充：12 字节 → 16 字符
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	plain := base64.RawURLEncoding.EncodeToString(raw)
	hash, err := auth.HashPassword(plain)
	if err != nil {
		return "", err
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.User{}).Where("id = ?", u.ID).Update("password_hash", hash).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Session{}, "user_id = ?", u.ID).Error
	})
	if err != nil {
		return "", err
	}
	return plain, nil
}
