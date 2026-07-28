package apitoken

import (
	"errors"
	"strings"
	"testing"

	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/auth"
	"github.com/yixian-huang/imgli/internal/service/settings"
)

func setup(t *testing.T) (*Service, *model.User) {
	db := model.TestDB(t)
	u, err := auth.New(db, settings.New(db)).Register("alice", "alice@img.li", "passw0rd", "")
	if err != nil {
		t.Fatal(err)
	}
	return New(db), u
}

func TestCreateListRevoke(t *testing.T) {
	svc, u := setup(t)

	plain, tok, err := svc.Create(u.ID, "picgo", "upload")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(plain, Prefix) || len(plain) < 40 {
		t.Errorf("明文格式: %q", plain)
	}
	if tok.TokenHash == plain || tok.TokenHash == "" {
		t.Error("库中必须存哈希而非明文")
	}

	list, err := svc.List(u.ID)
	if err != nil || len(list) != 1 || list[0].Name != "picgo" || list[0].Scope != "upload" {
		t.Fatalf("List: %+v, %v", list, err)
	}

	if _, _, err := svc.Create(u.ID, "bad", "root"); !errors.Is(err, ErrInvalidScope) {
		t.Errorf("非法 scope err = %v", err)
	}

	if err := svc.Revoke(u.ID, tok.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.Revoke(u.ID, tok.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("重复吊销 err = %v", err)
	}
	if err := svc.Revoke(u.ID+999, tok.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("他人 token err = %v", err)
	}
}

func TestUserByToken(t *testing.T) {
	svc, u := setup(t)
	plain, _, _ := svc.Create(u.ID, "picgo", "upload")

	got, scope, err := svc.UserByToken(plain)
	if err != nil || got == nil || got.ID != u.ID || scope != "upload" {
		t.Fatalf("UserByToken: %+v %q %v", got, scope, err)
	}
	// last_used_at 已更新
	list, _ := svc.List(u.ID)
	if list[0].LastUsedAt == nil {
		t.Error("last_used_at 应被更新")
	}

	if got, _, _ := svc.UserByToken("bl_garbage"); got != nil {
		t.Error("垃圾 token 应返回 nil")
	}
	if got, _, _ := svc.UserByToken("no-prefix"); got != nil {
		t.Error("无前缀应返回 nil")
	}
}
