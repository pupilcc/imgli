package moderation

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

const openAIMaxBytes = 20 << 20

// OpenAIScorer 经 OpenAI omni-moderation 接口打分。
type OpenAIScorer struct {
	APIKey string
	Client *http.Client
}

// Score 读图(≤20MB)→ base64 data URI → POST omni-moderation → 取 sexual 系最大分。
// imageURL 忽略。超限返回 (0,nil)+slog(降级不阻断)。
func (o *OpenAIScorer) Score(ctx context.Context, r io.Reader, mime, imageURL string) (float64, error) {
	data, over, err := readCapped(r, openAIMaxBytes)
	if err != nil {
		return 0, err
	}
	if over {
		slog.Warn("moderation: 图片超 OpenAI 20MB 上限,跳过", "size", len(data))
		return 0, nil
	}
	dataURI := "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
	body, _ := json.Marshal(map[string]any{
		"model": "omni-moderation-latest",
		"input": []any{map[string]any{"type": "image_url", "image_url": map[string]any{"url": dataURI}}},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/moderations", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.APIKey)
	resp, err := httpClientOr(o.Client).Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("moderation: openai 返回状态 %d", resp.StatusCode)
	}
	var out struct {
		Results []struct {
			CategoryScores map[string]float64 `json:"category_scores"`
		} `json:"results"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 256<<10)).Decode(&out); err != nil {
		return 0, fmt.Errorf("moderation: 解析 openai 响应失败: %w", err)
	}
	if len(out.Results) == 0 {
		return 0, errors.New("moderation: openai 响应缺 results")
	}
	cs := out.Results[0].CategoryScores
	score := cs["sexual"]
	if v := cs["sexual/minors"]; v > score {
		score = v
	}
	return score, nil
}
