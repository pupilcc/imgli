package imagesvc

import (
	"errors"
	"testing"
	"time"

	"github.com/yixian-huang/imgli/internal/model"
)

func TestGetOwnedReturnsRow(t *testing.T) {
	s, uid := setupSvc(t)
	// 取一条 key
	var img model.Image
	s.db.Where("user_id = ?", uid).First(&img)
	row, err := s.Get(uid, img.Key)
	if err != nil {
		t.Fatal(err)
	}
	if row.Img.Key != img.Key || row.File.ID == 0 {
		t.Fatalf("Get 应带 file: %+v", row)
	}
}

func TestGetForeignReturnsNotFound(t *testing.T) {
	s, _ := setupSvc(t)
	other := &model.User{Username: "u2", Email: "u2@img.li", GroupID: 1}
	s.db.Create(other)
	var img model.Image
	s.db.First(&img)
	if _, err := s.Get(other.ID, img.Key); !errors.Is(err, ErrNotFound) {
		t.Errorf("非属主应 ErrNotFound, got %v", err)
	}
}

func TestGetPublicSharePublicOK(t *testing.T) {
	s, _ := setupSvc(t)
	row, err := s.GetPublicShare("alphakey01")
	if err != nil {
		t.Fatal(err)
	}
	if row.Img.Visibility != "public" || row.Img.Name != "alpha" {
		t.Fatalf("%+v", row.Img)
	}
}

func TestGetPublicSharePrivateNotFound(t *testing.T) {
	s, _ := setupSvc(t)
	if _, err := s.GetPublicShare("bravokey01"); !errors.Is(err, ErrNotFound) {
		t.Errorf("private 应 ErrNotFound, got %v", err)
	}
}

func TestGetPublicSharePendingRejectedExpired(t *testing.T) {
	s, uid := setupSvc(t)
	past := time.Now().Add(-time.Hour)
	// pending public
	var pub model.Image
	if err := s.db.Where("key = ?", "alphakey01").First(&pub).Error; err != nil {
		t.Fatal(err)
	}
	s.db.Model(&pub).Update("status", "pending")
	if _, err := s.GetPublicShare(pub.Key); !errors.Is(err, ErrNotFound) {
		t.Errorf("pending: %v", err)
	}
	s.db.Model(&pub).Updates(map[string]any{"status": "rejected"})
	if _, err := s.GetPublicShare(pub.Key); !errors.Is(err, ErrNotFound) {
		t.Errorf("rejected: %v", err)
	}
	// expired public
	s.db.Model(&pub).Updates(map[string]any{"status": "normal", "expires_at": past})
	if _, err := s.GetPublicShare(pub.Key); !errors.Is(err, ErrNotFound) {
		t.Errorf("expired: %v", err)
	}
	_ = uid
}
