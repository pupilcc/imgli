package version

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// UpdateCheck 探测结果（无密钥）。
type UpdateCheck struct {
	Current         string    `json:"current"`
	Latest          string    `json:"latest,omitempty"`
	UpdateAvailable bool      `json:"update_available"`
	ReleaseURL      string    `json:"release_url,omitempty"`
	CheckedAt       time.Time `json:"checked_at"`
	Error           string    `json:"error,omitempty"`
}

// CheckLatestRelease 用 GitHub releases/latest 重定向解析最新 tag（与 install.sh 同思路，免 API token）。
// 先 HEAD，无 Location 时再 GET（部分网络/CDN 对 HEAD 不回跳转）。
func CheckLatestRelease(ctx context.Context, repo string, client *http.Client) UpdateCheck {
	if repo == "" {
		repo = DefaultReleaseRepo
	}
	out := UpdateCheck{
		Current:   Version,
		CheckedAt: time.Now().UTC(),
	}
	if client == nil {
		client = &http.Client{
			Timeout: 12 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}
	url := "https://github.com/" + repo + "/releases/latest"
	loc, status, err := latestRedirect(ctx, client, http.MethodHead, url)
	if err != nil || loc == "" {
		loc2, status2, err2 := latestRedirect(ctx, client, http.MethodGet, url)
		if err2 != nil {
			if err != nil {
				out.Error = err.Error()
			} else {
				out.Error = err2.Error()
			}
			return out
		}
		loc, status, err = loc2, status2, err2
	}
	if loc == "" {
		out.Error = fmt.Sprintf("unexpected status %d (no Location)", status)
		return out
	}
	// .../releases/tag/v0.5.1
	tag := loc
	if i := strings.LastIndex(loc, "/"); i >= 0 {
		tag = loc[i+1:]
	}
	tag = strings.TrimSpace(tag)
	if tag == "" {
		out.Error = "empty latest tag"
		return out
	}
	out.Latest = tag
	out.ReleaseURL = "https://github.com/" + repo + "/releases/tag/" + tag
	if Version == "dev" || Version == "" {
		out.UpdateAvailable = true
		return out
	}
	out.UpdateAvailable = CompareSemver(Version, tag) < 0
	return out
}

func latestRedirect(ctx context.Context, client *http.Client, method, url string) (loc string, status int, err error) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("User-Agent", "imgli-update-check/"+Version)
	res, err := client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer res.Body.Close()
	// drain a bit on GET so connections reuse cleanly
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 64<<10))
	return res.Header.Get("Location"), res.StatusCode, nil
}
