package imagesvc

import (
	"strings"
	"testing"

	"github.com/yixian-huang/imgli/internal/model"
)

func keysOf(s *Service, uid uint64) []string {
	var imgs []model.Image
	s.db.Where("user_id = ?", uid).Order("id").Find(&imgs)
	ks := make([]string, 0, len(imgs))
	for _, i := range imgs {
		ks = append(ks, i.Key)
	}
	return ks
}

func TestBatchDeletePartialSuccess(t *testing.T) {
	s, uid := setupSvc(t)
	ks := keysOf(s, uid)
	res, err := s.Batch(uid, "delete", append(ks, "ghostkey0000"), BatchOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 3 {
		t.Fatalf("应 3 条结果, got %d", len(res))
	}
	okCount := 0
	for _, r := range res {
		if r.OK {
			okCount++
		}
	}
	if okCount != 2 {
		t.Errorf("应 2 成功 1 失败(幽灵 key), got ok=%d", okCount)
	}
	trash, _, _ := s.TrashList(uid, "", 10)
	if len(trash) != 2 {
		t.Fatalf("批量软删应进回收站 2 张, got %d", len(trash))
	}
}

func TestBatchVisibilityAndMove(t *testing.T) {
	s, uid := setupSvc(t)
	alb := &model.Album{UserID: uid, Name: "A", Visibility: "private", ListInPlaza: true}
	s.db.Create(alb)
	ks := keysOf(s, uid)
	if _, err := s.Batch(uid, "visibility", ks, BatchOpts{Visibility: "private"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Batch(uid, "move", ks, BatchOpts{AlbumID: ptrI64(int64(alb.ID))}); err != nil {
		t.Fatal(err)
	}
	var cnt int64
	s.db.Model(&model.Image{}).Where("user_id = ? AND visibility = 'private' AND album_id = ?", uid, alb.ID).Count(&cnt)
	if cnt != 2 {
		t.Errorf("批量改可见性+移相册应作用 2 张, got %d", cnt)
	}
}

func TestBatchRenamePattern(t *testing.T) {
	s, uid := setupSvc(t)
	ks := keysOf(s, uid)
	res, err := s.Batch(uid, "rename", ks, BatchOpts{NamePattern: "trip_{n}"})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range res {
		if !r.OK {
			t.Fatalf("rename fail %s: %s", r.Key, r.Err)
		}
	}
	var imgs []model.Image
	s.db.Where("user_id = ?", uid).Order("id").Find(&imgs)
	if len(imgs) < 2 {
		t.Fatal("need 2 images")
	}
	if !strings.HasPrefix(imgs[0].Name, "trip_1.") {
		t.Errorf("first name = %q", imgs[0].Name)
	}
	if !strings.HasPrefix(imgs[1].Name, "trip_2.") {
		t.Errorf("second name = %q", imgs[1].Name)
	}
}

func TestBatchRenamePipelineAndPad(t *testing.T) {
	s, uid := setupSvc(t)
	var imgs []model.Image
	s.db.Where("user_id = ?", uid).Order("id").Find(&imgs)
	s.db.Model(&imgs[0]).Update("name", "raw_BrandX_file.png")
	res, err := s.Batch(uid, "rename", []string{imgs[0].Key}, BatchOpts{
		Find:              "BrandX",
		Replace:           "",
		ReplaceIgnoreCase: true,
		CleanSeparators:   true,
		NamePattern:       "shot_{n:03}",
		StartN:            12,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res[0].OK || res[0].Skipped {
		t.Fatalf("want ok rename, got %+v", res[0])
	}
	var got model.Image
	s.db.Where("key = ?", imgs[0].Key).First(&got)
	if got.Name != "shot_012.png" {
		t.Errorf("got %q want shot_012.png", got.Name)
	}
}

func TestBatchRenameTokensAlbumDate(t *testing.T) {
	s, uid := setupSvc(t)
	var imgs []model.Image
	s.db.Where("user_id = ?", uid).Order("id").Find(&imgs)
	s.db.Model(&imgs[0]).Updates(map[string]any{"name": "x.png", "ext": "png"})
	res, err := s.Batch(uid, "rename", []string{imgs[0].Key}, BatchOpts{
		NamePattern: "{album}_{original}_{n}",
		AlbumName:   "旅行",
		StartN:      1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res[0].OK {
		t.Fatalf("%+v", res[0])
	}
	var got model.Image
	s.db.Where("key = ?", imgs[0].Key).First(&got)
	if got.Name != "旅行_x_1.png" {
		t.Errorf("got %q", got.Name)
	}
}

func TestBatchRenameSkipUnchanged(t *testing.T) {
	s, uid := setupSvc(t)
	var imgs []model.Image
	s.db.Where("user_id = ?", uid).Order("id").Find(&imgs)
	s.db.Model(&imgs[0]).Updates(map[string]any{"name": "stable.png", "ext": "png"})
	res, err := s.Batch(uid, "rename", []string{imgs[0].Key}, BatchOpts{Find: "___nope___"})
	if err != nil {
		t.Fatal(err)
	}
	if !res[0].OK || !res[0].Skipped {
		t.Fatalf("want skipped, got %+v", res[0])
	}
}

func TestBatchRenameConflict(t *testing.T) {
	s, uid := setupSvc(t)
	var imgs []model.Image
	s.db.Where("user_id = ?", uid).Order("id").Find(&imgs)
	if len(imgs) < 2 {
		t.Fatal("need 2")
	}
	s.db.Model(&imgs[0]).Updates(map[string]any{"name": "a.png", "ext": "png"})
	s.db.Model(&imgs[1]).Updates(map[string]any{"name": "b.png", "ext": "png"})
	res, err := s.Batch(uid, "rename", []string{imgs[0].Key, imgs[1].Key}, BatchOpts{NamePattern: "same"})
	if err != nil {
		t.Fatal(err)
	}
	fail := 0
	for _, r := range res {
		if !r.OK && strings.Contains(r.Err, "conflict") {
			fail++
		}
	}
	if fail < 2 {
		t.Fatalf("want 2 conflicts, got %+v", res)
	}
}

func TestBatchRejectsUnknownAction(t *testing.T) {
	s, uid := setupSvc(t)
	if _, err := s.Batch(uid, "nuke", keysOf(s, uid), BatchOpts{}); err == nil {
		t.Error("未知 action 应报错")
	}
}
