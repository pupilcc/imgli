package moderation

// PolicyFromConfig 从扁平 Config 得到 Policy。
// 现网 action 仅有 pending|rejected：表示「超阈值」时的处置，对应 over_threshold=review 时的 ActionReview，
// 或 over_threshold=block 时的 ActionBlock。投影阶段默认 over_threshold=review，故 action → ActionReview。
func PolicyFromConfig(cfg Config) Policy {
	pol := DefaultPolicy()
	switch cfg.Action {
	case "rejected":
		// 站长若配置 action=rejected，语义是超阈值直接拒——用 block 路径更贴切，
		// 同时 ActionReview 也设为 rejected，避免仅 review 时落到 pending。
		pol.ActionBlock = "rejected"
		pol.ActionReview = "rejected"
	case "pending":
		pol.ActionReview = "pending"
		pol.ActionBlock = "rejected"
	}
	return pol
}

// overThresholdFromAction 扁平 action → 插件超阈值 Severity。
// 推荐默认 review（pending）；action=rejected 时投影为 block。
func overThresholdFromAction(action string) string {
	if action == "rejected" {
		return SeverityBlock
	}
	return SeverityReview
}

// BuildCheckersFromConfig 将 Config 投影为插件列表。
// - 总开关 enabled=false → 空（主 provider 与 OCR 均不跑）
// - 主 provider：thresholdChecker（与 PR1 相同）
// - ocr_keywords.enabled：追加 OCR+词表插件（可与主 provider 并行串行执行）
// 注意：OCR 单独开、主 enabled=false 时仍不跑——总开关管整条机审任务；
// 若将来要「只开 OCR」，可把总开关打开并把 provider 阈值调极高，或后续拆总开关语义。
func BuildCheckersFromConfig(cfg Config) []Checker {
	if !cfg.Enabled {
		return nil
	}
	name := cfg.Provider
	if name == "" {
		name = "webhook"
	}
	out := []Checker{
		WrapScorer(name, NewScorerFromConfig(cfg), cfg.Threshold, overThresholdFromAction(cfg.Action)),
	}
	if cfg.OCRKeywords.Enabled {
		out = append(out, NewOCRKeywordsChecker(cfg.OCRKeywords))
	}
	return out
}
