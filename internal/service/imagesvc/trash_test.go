package imagesvc

import (
	"errors"
	"testing"

	"github.com/yixian-huang/imgli/internal/model"
)

func TestSoftDeleteHidesFromListAndShowsInTrash(t *testing.T) {
	s, uid := setupSvc(t)
	var img model.Image
	s.db.Where("user_id = ?", uid).First(&img)
	if err := s.SoftDelete(uid, img.Key); err != nil {
		t.Fatal(err)
	}
	rows, _, _ := s.List(uid, Filter{}, "", 10)
	for _, r := range rows {
		if r.Img.Key == img.Key {
			t.Fatal("软删图不应出现在列表")
		}
	}
	trash, _, err := s.TrashList(uid, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range trash {
		if r.Img.Key == img.Key {
			found = true
		}
	}
	if !found {
		t.Fatal("软删图应出现在回收站")
	}
}

func TestSoftDeleteForeign404(t *testing.T) {
	s, _ := setupSvc(t)
	other := &model.User{Username: "z", Email: "z@img.li", GroupID: 1}
	s.db.Create(other)
	var img model.Image
	s.db.First(&img)
	if err := s.SoftDelete(other.ID, img.Key); !errors.Is(err, ErrNotFound) {
		t.Errorf("非属主软删应 ErrNotFound, got %v", err)
	}
}

func TestRestoreBringsBack(t *testing.T) {
	s, uid := setupSvc(t)
	var img model.Image
	s.db.Where("user_id = ?", uid).First(&img)
	s.SoftDelete(uid, img.Key)
	if err := s.Restore(uid, img.Key); err != nil {
		t.Fatal(err)
	}
	rows, _, _ := s.List(uid, Filter{}, "", 10)
	back := false
	for _, r := range rows {
		if r.Img.Key == img.Key {
			back = true
		}
	}
	if !back {
		t.Fatal("恢复后应回到列表")
	}
}
