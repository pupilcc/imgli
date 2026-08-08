package storagesvc

import (
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestValidatePathTemplate(t *testing.T) {
	if err := ValidatePathTemplate(""); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePathTemplate("{Y}/{m}/{d}/{uniqid}.{ext}"); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePathTemplate("{rand:8}.{ext}"); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePathTemplate("{digits:16}.png"); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePathTemplate("{Y}{m}{d}.{ext}"); !errors.Is(err, ErrPathTemplateInvalid) {
		t.Fatalf("no random: %v", err)
	}
	if err := ValidatePathTemplate("{rand:7}.{ext}"); !errors.Is(err, ErrPathTemplateInvalid) {
		t.Fatalf("rand too short: %v", err)
	}
	if err := ValidatePathTemplate("{digits:10}.{ext}"); !errors.Is(err, ErrPathTemplateInvalid) {
		t.Fatalf("digits too short: %v", err)
	}
	if err := ValidatePathTemplate("{hex:8}.{ext}"); !errors.Is(err, ErrPathTemplateInvalid) {
		t.Fatalf("hex too short: %v", err)
	}
	if err := ValidatePathTemplate("../{uniqid}.{ext}"); !errors.Is(err, ErrPathTemplateInvalid) {
		t.Fatalf("dotdot: %v", err)
	}
	if err := ValidatePathTemplate("/{uniqid}.{ext}"); !errors.Is(err, ErrPathTemplateInvalid) {
		t.Fatalf("abs: %v", err)
	}
	if err := ValidatePathTemplate("{foo}.{ext}"); !errors.Is(err, ErrPathTemplateInvalid) {
		t.Fatalf("unknown: %v", err)
	}
	if err := ValidatePathTemplate("{uniqid:12}.{ext}"); !errors.Is(err, ErrPathTemplateInvalid) {
		t.Fatalf("uniqid:N: %v", err)
	}
}

func TestRenderPathTokens(t *testing.T) {
	r := New(nil, nil)
	now := time.Date(2026, 8, 8, 15, 4, 5, 123456789, time.UTC)

	p, err := r.RenderPath("{Y}/{m}/{d}/{uniqid}.{ext}", "png", now)
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^2026/08/08/[0-9A-Za-z]{12}\.png$`).MatchString(p) {
		t.Errorf("default-like path=%q", p)
	}

	p, err = r.RenderPath("{Y}{m}{d}_{H}{M}{S}_{ms}_{rand:10}.{ext}", "jpg", now)
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^20260808_150405_123_[0-9A-Za-z]{10}\.jpg$`).MatchString(p) {
		t.Errorf("time path=%q", p)
	}

	p, err = r.RenderPath("{hex:12}.{ext}", "png", now)
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^[0-9a-f]{12}\.png$`).MatchString(p) {
		t.Errorf("hex path=%q", p)
	}

	p, err = r.RenderPath("{HEX:12}.{ext}", "png", now)
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^[0-9A-F]{12}\.png$`).MatchString(p) {
		t.Errorf("HEX path=%q", p)
	}

	p, err = r.RenderPath("{digits:16}.{ext}", "png", now)
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^[0-9]{16}\.png$`).MatchString(p) {
		t.Errorf("digits path=%q", p)
	}

	// repeated random tokens should differ
	p, err = r.RenderPath("{rand:8}_{rand:8}.{ext}", "png", now)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(strings.TrimSuffix(p, ".png"), "_")
	if len(parts) != 2 || parts[0] == parts[1] {
		// extremely unlikely to collide; fail if equal to catch "same value twice" bug
		if len(parts) == 2 && parts[0] == parts[1] {
			t.Errorf("repeated rand should differ: %q", p)
		}
	}
}

func TestRenderPathRejectsInvalid(t *testing.T) {
	r := New(nil, nil)
	_, err := r.RenderPath("{Y}.{ext}", "png", time.Now())
	if !errors.Is(err, ErrPathTemplateInvalid) {
		t.Fatalf("err=%v", err)
	}
}
