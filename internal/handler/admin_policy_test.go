package handler

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yixian-huang/imgli/internal/model"
)

func TestAdminPolicyDTOS3MasksSecret(t *testing.T) {
	p := &model.StoragePolicy{
		Name:   "s3pol",
		Driver: "s3",
		Config: map[string]string{
			"endpoint":          "s3.example.com",
			"region":            "us-east-1",
			"bucket":            "b",
			"access_key_id":     "AKIDEXAMPLE0000",
			"secret_access_key": "SECRETxxxx",
			"path_style":        "true",
		},
	}
	m := adminPolicyDTO(p)
	raw, ok := m["config"].(string)
	if !ok {
		t.Fatalf("config type %T", m["config"])
	}
	var cfg map[string]string
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg["secret_access_key"] != "****xxxx" {
		t.Errorf("secret 打码 = %q, want ****xxxx", cfg["secret_access_key"])
	}
	if cfg["access_key_id"] != "AKIDEXAMPLE0000" {
		t.Errorf("access_key_id 应明文: %q", cfg["access_key_id"])
	}
	if cfg["endpoint"] != "s3.example.com" {
		t.Errorf("endpoint 应明文: %q", cfg["endpoint"])
	}
	// 原对象 config 不应被 DTO 修改
	if p.Config["secret_access_key"] != "SECRETxxxx" {
		t.Errorf("DTO 不应改写原 secret: %q", p.Config["secret_access_key"])
	}
	if strings.Contains(raw, "SECRETxxxx") {
		t.Error("响应 config JSON 不应含明文 secret")
	}
}

func TestAdminPolicyDTOLocalNoMask(t *testing.T) {
	p := &model.StoragePolicy{
		Name:   "local",
		Driver: "local",
		Config: map[string]string{"root": "/data"},
	}
	m := adminPolicyDTO(p)
	raw := m["config"].(string)
	if !strings.Contains(raw, "/data") {
		t.Errorf("local config = %s", raw)
	}
}

func TestAdminPolicyDTOWebDAVMasksPassword(t *testing.T) {
	p := &model.StoragePolicy{
		Name:   "webdavpol",
		Driver: "webdav",
		Config: map[string]string{
			"endpoint": "https://dav.example.com/imgli",
			"username": "alice",
			"password": "s3cretpass",
		},
	}
	m := adminPolicyDTO(p)
	raw, ok := m["config"].(string)
	if !ok {
		t.Fatalf("config type %T", m["config"])
	}
	var cfg map[string]string
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg["password"] != "****pass" {
		t.Errorf("password 打码 = %q, want ****pass", cfg["password"])
	}
	if cfg["username"] != "alice" {
		t.Errorf("username 应明文: %q", cfg["username"])
	}
	if cfg["endpoint"] != "https://dav.example.com/imgli" {
		t.Errorf("endpoint 应明文: %q", cfg["endpoint"])
	}
	if p.Config["password"] != "s3cretpass" {
		t.Errorf("DTO 不应改写原 password: %q", p.Config["password"])
	}
	if strings.Contains(raw, "s3cretpass") {
		t.Error("响应 config JSON 不应含明文 password")
	}
}
