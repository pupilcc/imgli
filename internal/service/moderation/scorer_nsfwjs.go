package moderation

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// NSFWJSScorer 经 nsfwjs-api 兼容 endpoint 打分。
type NSFWJSScorer struct {
	Endpoint string
	APIKey   string
	Client   *http.Client
}

// Score POST 原图字节(流式)→ nsfwjs-api 响应,分=clamp(max(porn,hentai)+0.5*sexy,0,1)。imageURL 忽略。
func (n *NSFWJSScorer) Score(ctx context.Context, r io.Reader, mime, imageURL string) (float64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.Endpoint, r)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", mime)
	if n.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+n.APIKey)
	}
	resp, err := httpClientOr(n.Client).Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("moderation: nsfwjs 返回状态 %d", resp.StatusCode)
	}
	var out struct {
		Porn   float64 `json:"porn"`
		Hentai float64 `json:"hentai"`
		Sexy   float64 `json:"sexy"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&out); err != nil {
		return 0, fmt.Errorf("moderation: 解析 nsfwjs 响应失败: %w", err)
	}
	score := out.Porn
	if out.Hentai > score {
		score = out.Hentai
	}
	score += 0.5 * out.Sexy
	if score > 1 {
		score = 1
	}
	if score < 0 {
		score = 0
	}
	return score, nil
}
