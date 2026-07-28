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
	"strconv"
	"time"
)

const tencentMaxBytes = 10 << 20

// TencentScorer 经腾讯云 IMS ImageModeration 打分（TC3 签名，FileContent base64）。
type TencentScorer struct {
	SecretID  string
	SecretKey string
	Region    string
	Client    *http.Client
	now       func() time.Time
}

func (s *TencentScorer) nowOr() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

// Score 读图(≤10MB)→ base64 FileContent → POST IMS → Score/100。imageURL 忽略。超限 (0,nil)+slog。
func (s *TencentScorer) Score(ctx context.Context, r io.Reader, mime, imageURL string) (float64, error) {
	data, over, err := readCapped(r, tencentMaxBytes)
	if err != nil {
		return 0, err
	}
	if over {
		slog.Warn("moderation: 图片超腾讯云 10MB 上限,跳过", "size", len(data))
		return 0, nil
	}

	host := "ims.tencentcloudapi.com"
	payload, err := json.Marshal(map[string]any{
		"FileContent": base64.StdEncoding.EncodeToString(data),
	})
	if err != nil {
		return 0, err
	}

	t := s.nowOr()
	timestamp := strconv.FormatInt(t.Unix(), 10)
	date := t.UTC().Format("2006-01-02")
	auth := tencentTC3(s.SecretID, s.SecretKey, "ims", host, timestamp, date, payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://"+host, bytes.NewReader(payload))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", auth)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Host", host)
	req.Header.Set("X-TC-Action", "ImageModeration")
	req.Header.Set("X-TC-Version", "2020-12-29")
	req.Header.Set("X-TC-Region", s.Region)
	req.Header.Set("X-TC-Timestamp", timestamp)

	resp, err := httpClientOr(s.Client).Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("moderation: tencent 返回状态 %d", resp.StatusCode)
	}

	var out struct {
		Response struct {
			Score        *float64 `json:"Score"`
			Suggestion   string   `json:"Suggestion"`
			Label        string   `json:"Label"`
			LabelResults []struct {
				Label string  `json:"Label"`
				Score float64 `json:"Score"`
			} `json:"LabelResults"`
			Error *struct {
				Code    string `json:"Code"`
				Message string `json:"Message"`
			} `json:"Error"`
		} `json:"Response"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 256<<10)).Decode(&out); err != nil {
		return 0, fmt.Errorf("moderation: 解析 tencent 响应失败: %w", err)
	}
	if out.Response.Error != nil {
		return 0, fmt.Errorf("moderation: tencent Error %s: %s", out.Response.Error.Code, out.Response.Error.Message)
	}

	// Normal/Pass 是明确安全:Label=Normal 时顶层 Score 是"确信正常"的置信度,
	// 直接用 Score/100 会把安全图误判高分(codex 评审)。
	rp := out.Response
	if rp.Label == "Normal" || rp.Suggestion == "Pass" {
		return 0, nil
	}
	if rp.Score != nil {
		return *rp.Score / 100, nil
	}
	// 退回 LabelResults:仅非 Normal 风险标签取最大分。
	var maxScore float64
	found := false
	for _, lr := range rp.LabelResults {
		if lr.Label == "Normal" {
			continue
		}
		found = true
		if lr.Score > maxScore {
			maxScore = lr.Score
		}
	}
	// 成功响应却无任何可用评分字段:畸形,报错触发重试/告警而非静默判安全(codex 评审)。
	if !found {
		return 0, errors.New("moderation: tencent 响应缺可用评分字段")
	}
	return maxScore / 100, nil
}
