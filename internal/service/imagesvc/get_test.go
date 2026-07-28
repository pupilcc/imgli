package imagesvc

import (
	"errors"
	"testing"

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
