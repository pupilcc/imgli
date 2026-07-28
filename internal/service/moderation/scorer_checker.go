package moderation

import (
	"context"
	"fmt"
)

// thresholdChecker 将 Scorer + 阈值适配为 Checker。
// overThreshold: "review"（默认）| "block"——分数 ≥ threshold 时的 Severity。
type thresholdChecker struct {
	name          string
	scorer        Scorer
	threshold     float64
	overThreshold string
}

// Name 实现 Checker。
func (t *thresholdChecker) Name() string { return t.name }

// Check 打开原图、打分、映射 Severity。
func (t *thresholdChecker) Check(ctx context.Context, img ImageRef) (CheckResult, error) {
	if t.scorer == nil {
		return CheckResult{}, fmt.Errorf("moderation: plugin %s 无 scorer", t.name)
	}
	if img.Open == nil {
		return CheckResult{}, fmt.Errorf("moderation: plugin %s 无 Open", t.name)
	}
	rc, err := img.Open(ctx)
	if err != nil {
		return CheckResult{}, err
	}
	defer rc.Close()

	score, err := t.scorer.Score(ctx, rc, img.MIME, img.ImageURL)
	if err != nil {
		return CheckResult{}, err
	}
	sc := score
	res := CheckResult{
		Plugin:   t.name,
		Severity: SeverityNone,
		Score:    &sc,
	}
	if score >= t.threshold {
		if t.overThreshold == SeverityBlock || t.overThreshold == "block" {
			res.Severity = SeverityBlock
		} else {
			res.Severity = SeverityReview
		}
	}
	return res, nil
}

// WrapScorer 构造阈值型 Checker。overThreshold 空则 review。
func WrapScorer(name string, sc Scorer, threshold float64, overThreshold string) Checker {
	if overThreshold == "" {
		overThreshold = SeverityReview
	}
	return &thresholdChecker{
		name: name, scorer: sc, threshold: threshold, overThreshold: overThreshold,
	}
}
