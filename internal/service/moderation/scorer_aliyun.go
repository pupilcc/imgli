package moderation

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// AliyunScorer 经阿里云内容安全 ImageModeration 打分（ACS3 签名，按公网 imageURL）。
type AliyunScorer struct {
	AccessKeyID     string
	AccessKeySecret string
	Region          string
	Client          *http.Client
	now             func() time.Time
	nonce           func() string
}

func (s *AliyunScorer) nowOr() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

func (s *AliyunScorer) nonceOr() string {
	if s.nonce != nil {
		return s.nonce()
	}
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// Score 需非空 imageURL（公网图）；空 URL 降级 (0,nil)。分 = max(Result.Confidence)/100。
func (s *AliyunScorer) Score(ctx context.Context, r io.Reader, mime, imageURL string) (float64, error) {
	if imageURL == "" {
		slog.Warn("moderation: aliyun 需公网图片 URL,当前未提供")
		return 0, nil
	}

	host := "green-cip." + s.Region + ".aliyuncs.com"
	sp, _ := json.Marshal(map[string]string{"imageUrl": imageURL})
	payload, err := json.Marshal(map[string]any{
		"Service":           "baselineCheck",
		"ServiceParameters": string(sp),
	})
	if err != nil {
		return 0, err
	}

	timestamp := s.nowOr().UTC().Format("2006-01-02T15:04:05Z")
	nonce := s.nonceOr()
	auth := aliyunACS3(s.AccessKeyID, s.AccessKeySecret, host, "ImageModeration", "2022-03-02", nonce, timestamp, payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://"+host, bytes.NewReader(payload))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", auth)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Host", host)
	req.Header.Set("x-acs-action", "ImageModeration")
	req.Header.Set("x-acs-version", "2022-03-02")
	req.Header.Set("x-acs-content-sha256", sha256hex(payload))
	req.Header.Set("x-acs-date", timestamp)
	req.Header.Set("x-acs-signature-nonce", nonce)

	resp, err := httpClientOr(s.Client).Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("moderation: aliyun 返回状态 %d", resp.StatusCode)
	}

	var out struct {
		Code int `json:"Code"`
		Data struct {
			Result []struct {
				Label      string  `json:"Label"`
				Confidence float64 `json:"Confidence"`
			} `json:"Result"`
		} `json:"Data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 256<<10)).Decode(&out); err != nil {
		return 0, fmt.Errorf("moderation: 解析 aliyun 响应失败: %w", err)
	}
	if out.Code != 200 {
		return 0, fmt.Errorf("moderation: aliyun Code=%d", out.Code)
	}
	// 缺 Result 视为畸形响应报错(触发重试/告警),不静默当安全(codex 评审)。
	if len(out.Data.Result) == 0 {
		return 0, errors.New("moderation: aliyun 响应缺 Data.Result")
	}
	// 仅聚合风险标签:"nonLabel" 是"无风险"结果,其高置信度代表"确信安全",
	// 必须排除,否则安全图被误判高分(codex 评审)。全为 nonLabel → 0。
	var maxConf float64
	for _, item := range out.Data.Result {
		if item.Label == "nonLabel" {
			continue
		}
		if item.Confidence > maxConf {
			maxConf = item.Confidence
		}
	}
	return maxConf / 100, nil
}
