package moderation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode"
)

// OCRKeywordsChecker 外置 OCR + imgli 词表匹配（设计 PR3，默认 A 协议）。
//
// 协议：POST endpoint，body=原图像素流，Content-Type=mime；
// 可选 Authorization: Bearer <api_key>。
// 响应 200 JSON：{"text":"..."}；可选 {"hits":[...]} 会被忽略（词表以 imgli 为准）。
type OCRKeywordsChecker struct {
	Endpoint string
	APIKey   string
	Keywords []string
	OnHit    string // review | block；空=review
	Client   *http.Client
}

// Name 实现 Checker。
func (o *OCRKeywordsChecker) Name() string { return "ocr_keywords" }

// Check 拉 OCR 文本并做大小写不敏感子串匹配。
func (o *OCRKeywordsChecker) Check(ctx context.Context, img ImageRef) (CheckResult, error) {
	if img.Open == nil {
		return CheckResult{}, fmt.Errorf("moderation: ocr_keywords 无 Open")
	}
	rc, err := img.Open(ctx)
	if err != nil {
		return CheckResult{}, err
	}
	defer rc.Close()
	// 限制 20MB 与现 scorer 对齐，避免 OOM
	data, over, err := readCapped(rc, 20<<20)
	if err != nil {
		return CheckResult{}, err
	}
	if over {
		return CheckResult{Plugin: o.Name(), Severity: SeverityNone}, nil
	}

	text, err := o.fetchText(ctx, data, img.MIME)
	if err != nil {
		return CheckResult{}, err
	}
	hits := matchKeywords(text, o.Keywords)
	res := CheckResult{
		Plugin:   o.Name(),
		Severity: SeverityNone,
		Hits:     hits,
		Detail:   map[string]any{"text_len": len([]rune(text))},
	}
	// 文本过长不落 audit 全文，避免 audit 膨胀；命中时带 hits 即可。
	if len(hits) == 0 {
		return res, nil
	}
	if o.OnHit == SeverityBlock || o.OnHit == "block" {
		res.Severity = SeverityBlock
	} else {
		res.Severity = SeverityReview
	}
	return res, nil
}

func (o *OCRKeywordsChecker) fetchText(ctx context.Context, data []byte, mime string) (string, error) {
	if o.Endpoint == "" {
		return "", fmt.Errorf("moderation: ocr_keywords endpoint 为空")
	}
	if mime == "" {
		mime = "application/octet-stream"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.Endpoint, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", mime)
	if o.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+o.APIKey)
	}
	resp, err := httpClientOr(o.Client).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("moderation: ocr 返回状态 %d", resp.StatusCode)
	}
	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil {
		return "", fmt.Errorf("moderation: 解析 ocr 响应失败: %w", err)
	}
	return body.Text, nil
}

// matchKeywords 大小写不敏感子串匹配；关键词 trim 后空串忽略；去重保序。
func matchKeywords(text string, keywords []string) []string {
	if text == "" || len(keywords) == 0 {
		return nil
	}
	// 折叠空白，便于「关 键 词」类绕过弱化一点（仍是子串，不做分词）
	norm := strings.ToLower(compactSpace(text))
	// OCR 常在拉丁字母间插空格（IMGLIOC RSMOKE）；去空格再匹配一次
	normNS := strings.ReplaceAll(norm, " ", "")
	seen := map[string]struct{}{}
	var hits []string
	for _, kw := range keywords {
		k := strings.ToLower(strings.TrimSpace(kw))
		if k == "" {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		kNS := strings.ReplaceAll(k, " ", "")
		if strings.Contains(norm, k) || (kNS != "" && strings.Contains(normNS, kNS)) {
			seen[k] = struct{}{}
			hits = append(hits, strings.TrimSpace(kw))
		}
	}
	return hits
}

func compactSpace(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		prevSpace = false
		b.WriteRune(r)
	}
	return b.String()
}

// NewOCRKeywordsChecker 从配置构造；Enabled 须由调用方先判断。
func NewOCRKeywordsChecker(o OCRKeywordsConfig) *OCRKeywordsChecker {
	return &OCRKeywordsChecker{
		Endpoint: o.Endpoint,
		APIKey:   o.APIKey,
		Keywords: o.Keywords,
		OnHit:    o.OnHit,
	}
}
