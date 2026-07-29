package handler

import (
	"strings"
	"testing"

	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/bandwidth"
)

func TestBandwidthHardCapOnServe(t *testing.T) {
	fx := newServeFixture(t)
	// 默认组设极小月流量
	if err := fx.db.Model(&model.UserGroup{}).Where("id = ?", 1).
		Update("bandwidth_quota_month", int64(10)).Error; err != nil {
		t.Fatal(err)
	}
	// 属主已用满
	var img model.Image
	if err := fx.db.First(&img, fx.img.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := fx.db.Model(&model.User{}).Where("id = ?", *img.UserID).Updates(map[string]any{
		"bandwidth_used_month": 10,
		"bandwidth_period":     bandwidth.CurrentPeriod(),
	}).Error; err != nil {
		t.Fatal(err)
	}

	rec := fx.get("/i/"+fx.name, nil)
	if rec.Code != 429 {
		t.Fatalf("status=%d want 429 body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "BANDWIDTH EXCEEDED") {
		t.Errorf("body=%s", rec.Body.String())
	}

	rec = fx.get("/i/"+fx.name, map[string]string{"Accept": "application/json"})
	if rec.Code != 429 {
		t.Fatalf("json status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), CodeBandwidthExceeded) {
		t.Errorf("json body=%s", rec.Body.String())
	}
}

func TestBandwidthMetersOnServe(t *testing.T) {
	fx := newServeFixture(t)
	if err := fx.db.Model(&model.UserGroup{}).Where("id = ?", 1).
		Update("bandwidth_quota_month", int64(1<<30)).Error; err != nil {
		t.Fatal(err)
	}
	rec := fx.get("/i/"+fx.name, nil)
	if rec.Code != 200 {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var u model.User
	if err := fx.db.First(&u, *fx.img.UserID).Error; err != nil {
		t.Fatal(err)
	}
	if u.BandwidthPeriod != bandwidth.CurrentPeriod() {
		t.Errorf("period=%q", u.BandwidthPeriod)
	}
	if u.BandwidthUsedMonth != 12 { // file.Size=12
		t.Errorf("used=%d want 12", u.BandwidthUsedMonth)
	}
}
