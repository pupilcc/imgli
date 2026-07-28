package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// TestConfigPublicNoAuth GET /api/v1/config 挂在 RequireUser 之外——无 cookie/Bearer
// 也应 200（公开端点），响应含 site_name/registration_mode/guest_upload_enabled/guest，
// 且不泄露 api_key 等私密字段（对照 handler 层 internal/handler/config_test.go 的单测）。
func TestConfigPublicNoAuth(t *testing.T) {
	s := newTestServer(t)

	rec, e := doJSON(t, s, "GET", "/api/v1/config", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("无凭证 GET /config = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if !e.Status {
		t.Fatalf("status = false: %s", rec.Body.String())
	}

	var body struct {
		SiteName           string `json:"site_name"`
		RegistrationMode   string `json:"registration_mode"`
		GuestUploadEnabled bool   `json:"guest_upload_enabled"`
		Guest              *struct {
			MaxFileSize int64    `json:"max_file_size"`
			AllowedExts []string `json:"allowed_exts"`
			PerDay      int      `json:"per_day"`
		} `json:"guest"`
	}
	if err := json.Unmarshal(e.Data, &body); err != nil {
		t.Fatalf("decode data: %v, data=%s", err, e.Data)
	}
	if body.SiteName == "" {
		t.Errorf("site_name 为空")
	}
	if body.RegistrationMode != "open" {
		t.Errorf("registration_mode = %q, want open（播种默认）", body.RegistrationMode)
	}
	if body.Guest == nil || body.Guest.PerDay <= 0 {
		t.Errorf("guest 限额缺失: %+v", body.Guest)
	}

	if strings.Contains(rec.Body.String(), "api_key") {
		t.Errorf("响应体泄露 api_key: %s", rec.Body.String())
	}
}
