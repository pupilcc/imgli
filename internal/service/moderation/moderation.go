// Package moderation 机审：Scorer 接口 + 内置 WebhookScorer + moderate_image 任务处理器。
//
// 依赖方向是硬约束：本文件只 import model/settings/storagesvc/storage/linkbuilder
// （以及标准库），绝不 import upload/adminsvc/handler——upload 经 task.Runner 解耦
// （payload 只带 image_id），adminsvc/handler 反向依赖本包读写机审配置/落审计，import
// 本包即成环。
package moderation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"gorm.io/gorm"

	"github.com/yixian-huang/imgli/internal/linkbuilder"
	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/settings"
	"github.com/yixian-huang/imgli/internal/service/storagesvc"
)

// Scorer 给一段图片字节流打出 NSFW 分数 ∈ [0,1]（值越大越可能违规）。
// imageURL 为公网可访问的图片 URL，仅 aliyun 等按 URL 审核的服务商使用，其余实现可忽略。
type Scorer interface {
	Score(ctx context.Context, r io.Reader, mime, imageURL string) (float64, error)
}

// WebhookScorer 通过 POST 到管理员配置的可信 endpoint 打分（裁决 9）：
// body=图片原始字节，Content-Type=mime；APIKey 非空时附 Authorization: Bearer <key>；
// 期望 200 + JSON {"score":0.87}，非 200 或缺 score 字段均报错。
type WebhookScorer struct {
	Endpoint string
	APIKey   string
	Client   *http.Client
}

// httpClientOr 返回 c 或默认 15s 客户端（各 Scorer 共用）。
func httpClientOr(c *http.Client) *http.Client {
	if c != nil {
		return c
	}
	return &http.Client{Timeout: 15 * time.Second}
}

// readCapped 读至多 max 字节；若源还有更多（超限）返回 over=true。
func readCapped(r io.Reader, max int) (data []byte, over bool, err error) {
	data, err = io.ReadAll(io.LimitReader(r, int64(max)+1))
	if err != nil {
		return nil, false, err
	}
	if len(data) > max {
		return data[:max], true, nil
	}
	return data, false, nil
}

// httpClient 返回实际使用的 http.Client：显式设置了用显式的，否则用默认（15s 超时）。
func (w *WebhookScorer) httpClient() *http.Client {
	return httpClientOr(w.Client)
}

// Score 打分。imageURL 仅 aliyun 用，本实现忽略。
func (w *WebhookScorer) Score(ctx context.Context, r io.Reader, mime, imageURL string) (float64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.Endpoint, r)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", mime)
	if w.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+w.APIKey)
	}
	resp, err := w.httpClient().Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("moderation: webhook 返回状态 %d", resp.StatusCode)
	}
	var body struct {
		Score *float64 `json:"score"`
	}
	// 限流响应体（仓库惯例见 handler/respond.go DecodeJSON）：webhook endpoint 虽是
	// 管理员配置的可信目标，但被攻破或配错时不应让一个超大/无限响应把 worker 拖入 OOM。
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&body); err != nil {
		return 0, fmt.Errorf("moderation: 解析响应失败: %w", err)
	}
	if body.Score == nil {
		return 0, errors.New("moderation: 响应缺少 score 字段")
	}
	return *body.Score, nil
}

// NewScorerFromConfig 按配置构造 Scorer。未知 provider 回退 webhook。
func NewScorerFromConfig(cfg Config) Scorer {
	switch cfg.Provider {
	case "openai":
		return &OpenAIScorer{APIKey: cfg.APIKey}
	case "nsfwjs":
		return &NSFWJSScorer{Endpoint: cfg.Endpoint, APIKey: cfg.APIKey}
	case "aliyun":
		return &AliyunScorer{AccessKeyID: cfg.AccessKeyID, AccessKeySecret: cfg.AccessKeySecret, Region: cfg.Region}
	case "tencent":
		return &TencentScorer{SecretID: cfg.AccessKeyID, SecretKey: cfg.AccessKeySecret, Region: cfg.Region}
	default: // webhook
		return &WebhookScorer{Endpoint: cfg.Endpoint, APIKey: cfg.APIKey}
	}
}

// RejectNotifier 机审/人审拒绝后的可选通知（由 server 注入 mail，避免本包 import mail 成环风险由上层接线）。
type RejectNotifier func(img model.Image)

// Service 是 moderate_image 任务的处理器：读配置、取图、读存储原图、打分、写结果。
type Service struct {
	db  *gorm.DB
	st  *settings.Service
	res *storagesvc.Resolver
	// OnReject 可选；NotifyOnReject 且 status 写入 rejected 成功后调用（异步由调用方保证）。
	OnReject RejectNotifier
}

func New(db *gorm.DB, st *settings.Service, res *storagesvc.Resolver) *Service {
	return &Service{db: db, st: st, res: res}
}

// applyStatusDecision 尝试把超阈值命中的图片状态从 normal 置为 action（pending|rejected）。
// 列选择更新（单列 Update，不 touch 其余字段）+ RowsAffected 门禁：
//   - status='normal'：防覆盖人工已做出的决定（approve/reject 等）
//   - is_whitelisted=false：防覆盖任务在途时管理员刚做出的加白信任决定——加白本身不改
//     status，单靠 status='normal' 守卫堵不住"任务读到 status=normal 之后、状态决定写
//     之前，管理员把图加白"这个竞态窗口，故需单独守卫
//   - deleted_at IS NULL：防覆盖并发软删
//
// 返回实际改动行数：0 表示被上述任一条件抢先，调用方应静默视为成功（不覆盖既有决定）。
func (s *Service) applyStatusDecision(imageID uint64, action string) (int64, error) {
	res := s.db.Model(&model.Image{}).
		Where("id = ? AND status = ? AND is_whitelisted = ? AND deleted_at IS NULL", imageID, "normal", false).
		Update("status", action)
	return res.RowsAffected, res.Error
}

// loadConfig 读机审配置；未播种/未设置时回退 DefaultConfig（出厂即 disabled）。
// 以 DefaultConfig 为基底 Unmarshal，旧 JSON 缺字段保留默认（如 login_sample_rate=1）。
func (s *Service) loadConfig() (Config, error) {
	cfg := DefaultConfig()
	if err := s.st.Get(model.SettingModeration, &cfg); err != nil && !errors.Is(err, settings.ErrNotFound) {
		return Config{}, err
	}
	NormalizeConfig(&cfg)
	return cfg, nil
}

// LoadConfig 导出给 upload 抽检等只读场景。
func (s *Service) LoadConfig() (Config, error) { return s.loadConfig() }

// NotifyRejectIfConfigured 人审拒绝后调用：受 NotifyOnReject 开关约束。
func (s *Service) NotifyRejectIfConfigured(img model.Image) {
	if s.OnReject == nil {
		return
	}
	cfg, err := s.loadConfig()
	if err != nil || !cfg.NotifyOnReject {
		return
	}
	go s.OnReject(img)
}

// ModerateTask 处理 moderate_image 任务（供 task.Runner.Register 直接注册）。
// payload JSON: {"image_id":N}。
//
// 流程：读配置（disabled→跳过）→ 取图（不存在/已软删/已加白/status≠normal 均跳过）→
// 扁平 config 投影为 Checker 管道 → 串行检查 → Policy 合并 → 写 nsfw_score（max）→
// 非 normal 时 applyStatusDecision → 命中落 audit（含 results）。
// 单插件失败返回 err 交任务重试（与旧行为一致）；最终失败不拦图（裁决 10）。
func (s *Service) ModerateTask(ctx context.Context, payload string) error {
	var p struct {
		ImageID uint64 `json:"image_id"`
	}
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		return fmt.Errorf("moderation: payload 解析失败: %w", err)
	}

	cfg, err := s.loadConfig()
	if err != nil {
		return err
	}
	checkers := BuildCheckersFromConfig(cfg)
	if len(checkers) == 0 {
		return nil // disabled 或无插件
	}

	var img model.Image
	if err := s.db.First(&img, p.ImageID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil // 不存在或已软删（默认 scope 已排除）——跳过
		}
		return err
	}
	if img.IsWhitelisted || img.Status != "normal" {
		return nil // 已加白或已被人工处理过——跳过
	}

	var file model.File
	if err := s.db.First(&file, img.FileID).Error; err != nil {
		return err
	}
	var policy model.StoragePolicy
	if err := s.db.First(&policy, file.StoragePolicyID).Error; err != nil {
		return err
	}
	driver, err := s.res.Driver(&policy)
	if err != nil {
		return err
	}

	imageURL := linkbuilder.Build(s.res.LinkBase(&policy), img.Key, img.Ext, img.Name).URL
	ref := ImageRef{
		ImageID: img.ID, FileID: file.ID, MIME: file.MIME, ImageURL: imageURL,
		Open: func(ctx context.Context) (io.ReadCloser, error) {
			return driver.Open(ctx, file.Path)
		},
	}

	results, err := RunPipelineWithErrorPolicy(ctx, ref, checkers, cfg.OnPluginError)
	if err != nil {
		return err // 单插件失败 + open：重试（裁决 10：耗尽仍不拦图）
	}
	dec := Decide(results, PolicyFromConfig(cfg))

	// 有分则记 nsfw_score（兼容旧字段：取 max）；无分不改列。
	if dec.Score != nil {
		if err := s.db.Model(&model.Image{}).Where("id = ?", img.ID).
			Update("nsfw_score", *dec.Score).Error; err != nil {
			return err
		}
	}

	if !dec.Flagged {
		return nil
	}

	rows, err := s.applyStatusDecision(img.ID, dec.Status)
	if err != nil {
		return err
	}
	if rows == 0 {
		return nil // 被抢先处理（人工决定/加白/已删）——静默成功
	}

	// audit：保留 score/action 兼容字段，并附 results 供审核原因展示（PR2 可用）。
	detailMap := map[string]any{
		"image_id": img.ID, "key": img.Key, "action": dec.Status,
		"results": resultsForAudit(results),
	}
	if dec.Score != nil {
		detailMap["score"] = *dec.Score
	}
	detail, _ := json.Marshal(detailMap)
	if err := s.db.Create(&model.AuditLog{
		ActorType: "system", Action: "moderation_flag", Detail: string(detail),
	}).Error; err != nil {
		slog.Warn("moderation audit 落库失败", "image_id", img.ID, "err", err)
	}
	if dec.Status == "rejected" && cfg.NotifyOnReject && s.OnReject != nil {
		// 重新读图状态字段（含 UserID）
		var out model.Image
		if s.db.First(&out, img.ID).Error == nil {
			go s.OnReject(out)
		}
	}
	return nil
}

// resultsForAudit 把 CheckResult 收成可 JSON 序列化的轻量结构（去掉不可导出细节）。
func resultsForAudit(in []CheckResult) []map[string]any {
	out := make([]map[string]any, 0, len(in))
	for _, r := range in {
		m := map[string]any{"plugin": r.Plugin, "severity": r.Severity}
		if r.Score != nil {
			m["score"] = *r.Score
		}
		if len(r.Hits) > 0 {
			m["hits"] = r.Hits
		}
		if len(r.Labels) > 0 {
			m["labels"] = r.Labels
		}
		out = append(out, m)
	}
	return out
}
