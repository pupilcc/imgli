package adminsvc

import (
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/yixian-huang/imgli/internal/model"
)

var codeRe = regexp.MustCompile(`^IL-[A-HJ-NP-Z2-9]{4}-[A-HJ-NP-Z2-9]{4}$`)

func TestCreateInvites(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	codes, err := svc.CreateInvites(1, 5, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(codes) != 5 {
		t.Fatalf("应生成 5 张, got %d", len(codes))
	}
	seen := map[string]bool{}
	for _, c := range codes {
		if !codeRe.MatchString(c) {
			t.Errorf("码格式不符: %q", c)
		}
		if seen[c] {
			t.Errorf("码重复: %q", c)
		}
		seen[c] = true
	}
	var n int64
	db.Model(&model.InviteCode{}).Where("expires_at IS NULL").Count(&n)
	if n != 5 {
		t.Errorf("expires_in_days=0 应落 NULL 过期, got %d 条 NULL", n)
	}
	// 带有效期
	codes2, err := svc.CreateInvites(1, 1, 7)
	if err != nil || len(codes2) != 1 {
		t.Fatalf("带有效期生成失败: %v", err)
	}
	var ic model.InviteCode
	db.Where("code = ?", codes2[0]).First(&ic)
	if ic.ExpiresAt == nil || time.Until(*ic.ExpiresAt) < 6*24*time.Hour {
		t.Errorf("expires_at 应约 7 天后, got %v", ic.ExpiresAt)
	}
	// 数量边界
	if _, err := svc.CreateInvites(1, 0, 0); !errors.Is(err, ErrInviteCountInvalid) {
		t.Errorf("count=0 err = %v, want ErrInviteCountInvalid", err)
	}
	if _, err := svc.CreateInvites(1, 101, 0); !errors.Is(err, ErrInviteCountInvalid) {
		t.Errorf("count=101 err = %v, want ErrInviteCountInvalid", err)
	}
}

func TestListInvitesStatusFilter(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	past := time.Now().Add(-time.Hour)
	uid := uint64(9)
	now := time.Now()
	db.Create(&model.InviteCode{Code: "IL-UNUS-ED22", CreatedBy: 1})
	db.Create(&model.InviteCode{Code: "IL-EXPD-RD22", CreatedBy: 1, ExpiresAt: &past})
	db.Create(&model.InviteCode{Code: "IL-USED-DD22", CreatedBy: 1, UsedBy: &uid, UsedAt: &now})

	for _, tc := range []struct {
		status string
		want   string
	}{{"unused", "IL-UNUS-ED22"}, {"expired", "IL-EXPD-RD22"}, {"used", "IL-USED-DD22"}} {
		rows, total, err := svc.ListInvites(tc.status, 1, 50)
		if err != nil || total != 1 || len(rows) != 1 || rows[0].Invite.Code != tc.want {
			t.Errorf("status=%s: rows=%v total=%d err=%v, want 恰含 %s", tc.status, len(rows), total, err, tc.want)
		}
		if len(rows) == 1 && rows[0].Status != tc.status {
			t.Errorf("status=%s: row.Status = %q, want %q", tc.status, rows[0].Status, tc.status)
		}
	}
	rows, total, _ := svc.ListInvites("", 1, 50)
	if total != 3 || len(rows) != 3 {
		t.Errorf("全部: total=%d len=%d, want 3", total, len(rows))
	}
}

func TestRevokeInvite(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	uid := uint64(9)
	now := time.Now()
	db.Create(&model.InviteCode{ID: 1, Code: "IL-FREE-EE22", CreatedBy: 1})
	db.Create(&model.InviteCode{ID: 2, Code: "IL-TAKE-EN22", CreatedBy: 1, UsedBy: &uid, UsedAt: &now})

	code, err := svc.RevokeInvite(1)
	if err != nil {
		t.Fatalf("未用码应可撤销: %v", err)
	}
	if code != "IL-FREE-EE22" {
		t.Errorf("返回 code = %q, want IL-FREE-EE22", code)
	}
	var n int64
	db.Model(&model.InviteCode{}).Where("id = 1").Count(&n)
	if n != 0 {
		t.Error("撤销应删除记录")
	}
	if _, err := svc.RevokeInvite(2); !errors.Is(err, ErrInviteUsed) {
		t.Errorf("已用码 err = %v, want ErrInviteUsed", err)
	}
	if _, err := svc.RevokeInvite(404); !errors.Is(err, ErrInviteNotFound) {
		t.Errorf("不存在 err = %v, want ErrInviteNotFound", err)
	}
}
