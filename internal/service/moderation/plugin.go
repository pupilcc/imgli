package moderation

import (
	"context"
	"io"
)

// 插件严重度：建议强度，非最终 images.status。
const (
	SeverityNone   = "none"   // 干净
	SeverityReview = "review" // 建议进待审（默认映射 pending）
	SeverityBlock  = "block"  // 建议直接处置（默认映射 rejected）
)

// CheckResult 单个检查插件的输出。
type CheckResult struct {
	Plugin   string             // 稳定 id：nsfwjs | webhook | tencent | ...
	Severity string             // none | review | block
	Score    *float64           // 可选 [0,1]
	Labels   map[string]float64 // 可选细分类
	Hits     []string           // 词表/规则命中
	Detail   map[string]any     // 自由扩展（audit 时截断）
}

// ImageRef 供 Checker 取图，避免依赖 upload 包。
type ImageRef struct {
	ImageID  uint64
	FileID   uint64
	MIME     string
	ImageURL string
	Open     func(ctx context.Context) (io.ReadCloser, error)
}

// Checker 机审插件。
type Checker interface {
	Name() string
	Check(ctx context.Context, img ImageRef) (CheckResult, error)
}

// Decision Pipeline 合并结果。
type Decision struct {
	Status   string   // normal | pending | rejected
	Score    *float64 // 各插件 Score 的 max；无则 nil
	Flagged  bool     // Status != normal
	Results  []CheckResult
	Action   string // 实际写入 status 的值（与 Status 相同，便于 audit）
}

// Policy 合并规则。
type Policy struct {
	ActionReview string // 默认 pending
	ActionBlock  string // 默认 rejected
}

// DefaultPolicy 与现网 action=pending 语义对齐：超阈值 → review → pending。
func DefaultPolicy() Policy {
	return Policy{ActionReview: "pending", ActionBlock: "rejected"}
}

// Decide 合并插件结果：block 优先于 review；Score 取 max。
// 无 review/block 时 Status=normal。
func Decide(results []CheckResult, pol Policy) Decision {
	if pol.ActionReview == "" {
		pol.ActionReview = "pending"
	}
	if pol.ActionBlock == "" {
		pol.ActionBlock = "rejected"
	}
	d := Decision{Status: "normal", Results: results}
	var maxScore *float64
	hasReview, hasBlock := false, false
	for i := range results {
		r := &results[i]
		if r.Score != nil {
			if maxScore == nil || *r.Score > *maxScore {
				v := *r.Score
				maxScore = &v
			}
		}
		switch r.Severity {
		case SeverityBlock:
			hasBlock = true
		case SeverityReview:
			hasReview = true
		}
	}
	d.Score = maxScore
	switch {
	case hasBlock:
		d.Status = pol.ActionBlock
		d.Flagged = true
	case hasReview:
		d.Status = pol.ActionReview
		d.Flagged = true
	}
	d.Action = d.Status
	return d
}

// RunPipeline 串行执行 checkers。
// onError: "open"（默认）| "review"。
//   - open + 单插件失败 → 返回 error（任务重试，现网语义）
//   - open + 多插件失败 → 跳过该插件
//   - review + 任一插件失败 → 合成 SeverityReview，不返回 error（避免无限重试）
func RunPipeline(ctx context.Context, img ImageRef, checkers []Checker) ([]CheckResult, error) {
	return RunPipelineWithErrorPolicy(ctx, img, checkers, "open")
}

// RunPipelineWithErrorPolicy 同 RunPipeline，显式 onError。
func RunPipelineWithErrorPolicy(ctx context.Context, img ImageRef, checkers []Checker, onError string) ([]CheckResult, error) {
	if onError == "" {
		onError = "open"
	}
	if len(checkers) == 0 {
		return nil, nil
	}
	out := make([]CheckResult, 0, len(checkers))
	single := len(checkers) == 1
	for _, c := range checkers {
		if c == nil {
			continue
		}
		r, err := c.Check(ctx, img)
		if err != nil {
			if onError == "review" {
				name := c.Name()
				out = append(out, CheckResult{
					Plugin:   name,
					Severity: SeverityReview,
					Detail:   map[string]any{"error": true, "message": err.Error()},
				})
				continue
			}
			if single {
				return nil, err
			}
			// 多插件 fail-open：跳过该插件
			continue
		}
		if r.Plugin == "" {
			r.Plugin = c.Name()
		}
		if r.Severity == "" {
			r.Severity = SeverityNone
		}
		out = append(out, r)
	}
	return out, nil
}
