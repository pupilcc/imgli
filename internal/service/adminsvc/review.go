package adminsvc

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/yixian-huang/imgli/internal/model"
)

// ErrInvalidAction action 不是 approve|reject。
var ErrInvalidAction = errors.New("action 仅支持 approve|reject")

// ErrNotPending 图片存在但当前不是待审（pending）状态，不可裁决。
var ErrNotPending = errors.New("仅待审图片可裁决")

// ErrTooManyKeys 批量裁决的 keys 数量超过上限。
var ErrTooManyKeys = errors.New("keys 数量超过上限 100")

// maxReviewBatchKeys 批量裁决单次上限（裁决 12）。
const maxReviewBatchKeys = 100

// ListReview 待审队列：status=pending 的全站图片，复用 Task 3 的 ImageRow/hydrate 基建
// （含属主/分数/物理文件），按 id 倒序。
func (s *Service) ListReview(page, limit int) ([]ImageRow, int64, error) {
	return s.ListImages(0, "pending", 0, "live", page, limit)
}

// reviewTargetStatus 把 approve|reject 映射为目标 status；action 非法返回空字符串。
func reviewTargetStatus(action string) string {
	switch action {
	case "approve":
		return "normal"
	case "reject":
		return "rejected"
	default:
		return ""
	}
}

// Decide 对单张图片做人工裁决：approve→normal，reject→rejected。
//
// 仅 pending 图可决：单语句列更新 WHERE key=? AND status='pending' AND deleted_at IS NULL
// + RowsAffected 门禁——先查再写会在查/写之间引入 TOCTOU 窗口（期间图可能被并发裁决/
// 加白/软删）；改为写后 0 行时才补查一次，仅用于区分错因（图不存在 vs 存在但非
// pending），补查结果不参与是否成功的决策——写句本身仍是唯一权威。
func (s *Service) Decide(key, action string) (*model.Image, error) {
	newStatus := reviewTargetStatus(action)
	if newStatus == "" {
		return nil, ErrInvalidAction
	}
	res := s.db.Model(&model.Image{}).
		Where("key = ? AND status = ? AND deleted_at IS NULL", key, "pending").
		Update("status", newStatus)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		var existing model.Image
		err := s.db.Where("key = ? AND deleted_at IS NULL", key).First(&existing).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			return nil, ErrImageNotFound
		case err != nil:
			return nil, err
		default:
			return nil, ErrNotPending
		}
	}
	return s.decideRefetch(key)
}

// decideRefetch 取回状态更新已提交（RowsAffected>=1）的那一行，供 Decide 的成功路径
// 构造返回值。Unscoped 取回——若在 UPDATE 与此处之间该图被并发软删（如
// AdminSoftDelete），默认 scope 会因 deleted_at 非空而查不到，误把已成功的裁决报成
// 500 且漏审计（同 AdminSoftDelete 取回已软删行供 audit 用的写法，见 images.go）。
// 抽成独立方法是为了让回归测试直接锁定这条查询本身（而非重放一份平行 SQL）——
// 把 Unscoped 误改回默认 scope 时，测试必须变红。
func (s *Service) decideRefetch(key string) (*model.Image, error) {
	var img model.Image
	if err := s.db.Unscoped().Where("key = ?", key).First(&img).Error; err != nil {
		return nil, err
	}
	return &img, nil
}

// NSFWScoreByKey 轻量取单图的 nsfw_score（供批量裁决后 audit detail 用），
// 避免为此单个字段调用 GetImageRow 付出 JOIN+hydrate（files/storage_policies 的
// 额外 IN 查询）的开销。不存在时返回 gorm.ErrRecordNotFound，调用方按 best-effort
// 容错（audit 场景下取不到分数不阻断请求）。
func (s *Service) NSFWScoreByKey(key string) (*float64, error) {
	var img model.Image
	if err := s.db.Select("nsfw_score").Where("key = ?", key).First(&img).Error; err != nil {
		return nil, err
	}
	return img.NSFWScore, nil
}

// NSFWScoresByKeys 批量取 nsfw_score；不存在的 key 不在 map 中。一次 IN 查询。
func (s *Service) NSFWScoresByKeys(keys []string) (map[string]*float64, error) {
	out := make(map[string]*float64, len(keys))
	if len(keys) == 0 {
		return out, nil
	}
	var rows []struct {
		Key       string
		NSFWScore *float64
	}
	if err := s.db.Model(&model.Image{}).
		Select("key", "nsfw_score").
		Where("key IN ?", keys).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.Key] = r.NSFWScore
	}
	return out, nil
}

// ImagesByKeys 批量取图片行（默认 scope，不含软删）；一次 IN 查询。
// 供批量裁决后 reject 通知等路径避免逐 key GetImageRow。
func (s *Service) ImagesByKeys(keys []string) (map[string]model.Image, error) {
	out := make(map[string]model.Image, len(keys))
	if len(keys) == 0 {
		return out, nil
	}
	var imgs []model.Image
	if err := s.db.Where("key IN ?", keys).Find(&imgs).Error; err != nil {
		return nil, err
	}
	for i := range imgs {
		out[imgs[i].Key] = imgs[i]
	}
	return out, nil
}

// ModerationTrigger 审核队列展示用的机审触发摘要（来自 moderation_flag audit）。
type ModerationTrigger struct {
	Plugin   string   `json:"plugin"`
	Severity string   `json:"severity"`
	Score    *float64 `json:"score,omitempty"`
	Hits     []string `json:"hits,omitempty"`
}

// ModerationTriggersByKeys 批量取每 key 最近一条 system moderation_flag 的 results。
// 旧 audit 仅有 score 时合成一条 plugin=legacy 的触发项。查不到则 key 不在 map 中。
// 一次查询：OR LIKE 所有 key + id 降序，内存里 first-wins 取每 key 最新日志。
func (s *Service) ModerationTriggersByKeys(keys []string) (map[string][]ModerationTrigger, error) {
	out := make(map[string][]ModerationTrigger, len(keys))
	if len(keys) == 0 {
		return out, nil
	}
	// 去重保序
	seen := make(map[string]struct{}, len(keys))
	uniq := make([]string, 0, len(keys))
	for _, k := range keys {
		if k == "" {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		uniq = append(uniq, k)
	}
	if len(uniq) == 0 {
		return out, nil
	}
	// detail 含 "key":"<key>"；key 为 base62 无引号，LIKE 安全。
	q := s.db.Where("action = ? AND actor_type = ?", "moderation_flag", "system")
	ors := make([]string, 0, len(uniq))
	args := make([]any, 0, len(uniq))
	for _, key := range uniq {
		ors = append(ors, "detail LIKE ?")
		args = append(args, fmt.Sprintf(`%%"key":"%s"%%`, key))
	}
	q = q.Where("("+strings.Join(ors, " OR ")+")", args...)
	var logs []model.AuditLog
	// 多取一些以防同 key 多条；按 id 降序后每 key 只保留第一条。
	if err := q.Order("id DESC").Limit(len(uniq) * 3).Find(&logs).Error; err != nil {
		return nil, err
	}
	for i := range logs {
		key := moderationFlagKey(logs[i].Detail)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; !ok {
			continue
		}
		if _, have := out[key]; have {
			continue // 已是更新的一条
		}
		trigs := parseModerationFlagTriggers(logs[i].Detail)
		if len(trigs) > 0 {
			out[key] = trigs
		}
	}
	return out, nil
}

// moderationFlagKey 从 audit detail 抽出 key 字段。
func moderationFlagKey(detail string) string {
	if strings.TrimSpace(detail) == "" {
		return ""
	}
	var raw struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal([]byte(detail), &raw); err != nil {
		return ""
	}
	return raw.Key
}

// parseModerationFlagTriggers 解析 moderation_flag detail。
// 新格式: {results:[{plugin,severity,score,hits}], score, action, key}
// 旧格式: {key, score, action} → 合成一条 legacy 触发。
func parseModerationFlagTriggers(detail string) []ModerationTrigger {
	if strings.TrimSpace(detail) == "" {
		return nil
	}
	var raw struct {
		Score   *float64 `json:"score"`
		Results []struct {
			Plugin   string   `json:"plugin"`
			Severity string   `json:"severity"`
			Score    *float64 `json:"score"`
			Hits     []string `json:"hits"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(detail), &raw); err != nil {
		return nil
	}
	if len(raw.Results) > 0 {
		out := make([]ModerationTrigger, 0, len(raw.Results))
		for _, r := range raw.Results {
			plugin := r.Plugin
			if plugin == "" {
				plugin = "unknown"
			}
			sev := r.Severity
			if sev == "" {
				sev = "review"
			}
			out = append(out, ModerationTrigger{
				Plugin: plugin, Severity: sev, Score: r.Score, Hits: r.Hits,
			})
		}
		return out
	}
	if raw.Score != nil {
		return []ModerationTrigger{{
			Plugin: "legacy", Severity: "review", Score: raw.Score,
		}}
	}
	return nil
}

// BatchResult 审核批量裁决单键结果。adminsvc 自定义类型（不复用 imagesvc.BatchResult），
// 避免 adminsvc 反向 import imagesvc 造成域间耦合。
type BatchResult struct {
	Key   string `json:"key"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// DecideBatch 批量裁决：action 先整体校验一次（ErrInvalidAction）、keys 数量上限 100
// （ErrTooManyKeys），随后逐项调用 Decide，单项失败不影响其余项（部分成功）。
func (s *Service) DecideBatch(keys []string, action string) ([]BatchResult, error) {
	if reviewTargetStatus(action) == "" {
		return nil, ErrInvalidAction
	}
	if len(keys) > maxReviewBatchKeys {
		return nil, ErrTooManyKeys
	}
	out := make([]BatchResult, 0, len(keys))
	for _, k := range keys {
		_, err := s.Decide(k, action)
		br := BatchResult{Key: k, OK: err == nil}
		if err != nil {
			br.Error = err.Error()
		}
		out = append(out, br)
	}
	return out, nil
}
