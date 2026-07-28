package auth_test

import (
	"errors"
	"testing"

	"gorm.io/gorm"

	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/auth"
	"github.com/yixian-huang/imgli/internal/service/settings"
)

// setupPrefs 造用户(默认组)+他人相册,返回 (db, svc, 用户, 他人相册ID, 组允许策略ID)
func setupPrefs(t *testing.T) (*gorm.DB, *auth.Service, *model.User, uint64, uint64) {
	t.Helper()
	db := model.TestDB(t)
	svc := auth.New(db, settings.New(db))
	u, err := svc.Register("alice", "alice@img.li", "passw0rd1", "")
	if err != nil {
		t.Fatal(err)
	}
	other, err := svc.Register("bob", "bob@img.li", "passw0rd1", "")
	if err != nil {
		t.Fatal(err)
	}
	alb := model.Album{UserID: other.ID, Name: "bob 的相册"}
	if err := db.Create(&alb).Error; err != nil {
		t.Fatal(err)
	}
	var g model.UserGroup
	if err := db.First(&g, u.GroupID).Error; err != nil {
		t.Fatal(err)
	}
	if len(g.AllowedPolicyIDs) == 0 {
		t.Fatal("默认组应有允许策略(播种)")
	}
	return db, svc, u, alb.ID, g.AllowedPolicyIDs[0]
}

func TestUpdatePreferencesOK(t *testing.T) {
	db, svc, u, _, pid := setupPrefs(t)
	p := model.Preferences{DefaultVisibility: "private", DefaultPolicyID: &pid, AutoCopyFormat: "markdown", Lang: "en"}
	if err := svc.UpdatePreferences(u.ID, p); err != nil {
		t.Fatalf("合法偏好应保存: %v", err)
	}
	var fresh model.User
	if err := db.First(&fresh, u.ID).Error; err != nil {
		t.Fatal(err)
	}
	if fresh.Preferences.DefaultVisibility != "private" ||
		fresh.Preferences.DefaultPolicyID == nil || *fresh.Preferences.DefaultPolicyID != pid ||
		fresh.Preferences.AutoCopyFormat != "markdown" ||
		fresh.Preferences.Lang != "en" {
		t.Errorf("落库偏好不符: %+v", fresh.Preferences)
	}
	// lang 空串合法(跟随前端)
	p.Lang = ""
	if err := svc.UpdatePreferences(u.ID, p); err != nil {
		t.Fatalf("lang 空应合法: %v", err)
	}
	if err := db.First(&fresh, u.ID).Error; err != nil {
		t.Fatal(err)
	}
	if fresh.Preferences.Lang != "" {
		t.Errorf("lang 空应落库: %+v", fresh.Preferences)
	}
	// lang=zh 往返
	p.Lang = "zh"
	if err := svc.UpdatePreferences(u.ID, p); err != nil {
		t.Fatalf("lang=zh 应合法: %v", err)
	}
	if err := db.First(&fresh, u.ID).Error; err != nil {
		t.Fatal(err)
	}
	if fresh.Preferences.Lang != "zh" {
		t.Errorf("lang=zh 落库: got %q", fresh.Preferences.Lang)
	}
}

func TestUpdatePreferencesInvalid(t *testing.T) {
	_, svc, u, otherAlb, _ := setupPrefs(t)
	bad := uint64(99999)
	cases := []model.Preferences{
		{DefaultVisibility: "friends"}, // 非法可见性
		{AutoCopyFormat: "rtf"},        // 非法格式
		{Lang: "fr"},                   // 非法语言
		{Lang: "zh-CN"},                // 非精确 zh/en
		{DefaultAlbumID: &otherAlb},    // 他人相册
		{DefaultAlbumID: &bad},         // 不存在相册
		{DefaultPolicyID: &bad},        // 不在组允许
	}
	for i, p := range cases {
		if err := svc.UpdatePreferences(u.ID, p); !errors.Is(err, auth.ErrInvalidInput) {
			t.Errorf("case %d 应 ErrInvalidInput, got %v", i, err)
		}
	}
}

func TestUserPolicies(t *testing.T) {
	db, svc, u, _, _ := setupPrefs(t)
	// 造两个 enabled 策略 P2/P1（播种 id=1 已是 P 本地；再建 P2）
	p2 := &model.StoragePolicy{
		Name: "extra-p2", Driver: "local", Enabled: true,
		Config: map[string]string{"root": t.TempDir()},
	}
	if err := db.Create(p2).Error; err != nil {
		t.Fatal(err)
	}
	var g model.UserGroup
	if err := db.First(&g, u.GroupID).Error; err != nil {
		t.Fatal(err)
	}
	// allowed = [P2, P1] 保序
	g.AllowedPolicyIDs = []uint64{p2.ID, 1}
	if err := db.Model(&g).Select("allowed_policy_ids").Updates(&g).Error; err != nil {
		t.Fatal(err)
	}

	list, err := svc.UserPolicies(u)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].ID != p2.ID || list[1].ID != 1 {
		t.Fatalf("应保序 [P2,P1], got %+v", list)
	}
	if list[0].Name == "" || list[1].Name == "" {
		t.Errorf("Name 应填充: %+v", list)
	}

	// P1 置 enabled=false → 仅 [P2]
	if err := db.Model(&model.StoragePolicy{}).Where("id = ?", 1).Update("enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	list, err = svc.UserPolicies(u)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != p2.ID {
		t.Errorf("disabled 应过滤, got %+v", list)
	}
}

func TestUpdatePreferencesWatermark(t *testing.T) {
	db, svc, u, _, _ := setupPrefs(t)

	// 全零值(缺省)合法
	if err := svc.UpdatePreferences(u.ID, model.Preferences{}); err != nil {
		t.Fatalf("全零偏好应合法: %v", err)
	}

	// 合法 watermark
	p := model.Preferences{
		Watermark: model.WatermarkPref{
			Enabled: true, Position: "tl", Opacity: 0.8, Margin: 32,
		},
	}
	if err := svc.UpdatePreferences(u.ID, p); err != nil {
		t.Fatalf("合法 watermark 应保存: %v", err)
	}
	var fresh model.User
	if err := db.First(&fresh, u.ID).Error; err != nil {
		t.Fatal(err)
	}
	w := fresh.Preferences.Watermark
	if !w.Enabled || w.Position != "tl" || w.Opacity != 0.8 || w.Margin != 32 {
		t.Errorf("落库 watermark 不符: %+v", w)
	}

	// 非法矩阵
	bads := []model.WatermarkPref{
		{Position: "xx"},
		{Opacity: 1.5},
		{Opacity: -0.1},
		{Margin: 300},
		{Margin: -1},
	}
	for i, wm := range bads {
		err := svc.UpdatePreferences(u.ID, model.Preferences{Watermark: wm})
		if !errors.Is(err, auth.ErrInvalidInput) {
			t.Errorf("bad[%d]=%+v: err=%v want ErrInvalidInput", i, wm, err)
		}
	}
}
