package imagesvc

import (
	"testing"

	"github.com/yixian-huang/imgli/internal/model"
)

func keysOf(s *Service, uid uint64) []string {
	var imgs []model.Image
	s.db.Where("user_id = ?", uid).Find(&imgs)
	ks := make([]string, 0, len(imgs))
	for _, i := range imgs {
		ks = append(ks, i.Key)
	}
	return ks
}

func TestBatchDeletePartialSuccess(t *testing.T) {
	s, uid := setupSvc(t)
	ks := keysOf(s, uid) // 2 个
	res, err := s.Batch(uid, "delete", append(ks, "ghostkey0000"), "", nil)
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
	// 都进回收站
	trash, _, _ := s.TrashList(uid, "", 10)
	if len(trash) != 2 {
		t.Errorf("批量软删应进回收站 2 张, got %d", len(trash))
	}
}

func TestBatchVisibilityAndMove(t *testing.T) {
	s, uid := setupSvc(t)
	alb := &model.Album{UserID: uid, Name: "A", Visibility: "private"}
	s.db.Create(alb)
	ks := keysOf(s, uid)
	if _, err := s.Batch(uid, "visibility", ks, "private", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Batch(uid, "move", ks, "", ptrI64(int64(alb.ID))); err != nil {
		t.Fatal(err)
	}
	var cnt int64
	s.db.Model(&model.Image{}).Where("user_id = ? AND visibility = 'private' AND album_id = ?", uid, alb.ID).Count(&cnt)
	if cnt != 2 {
		t.Errorf("批量改可见性+移相册应作用 2 张, got %d", cnt)
	}
}

func TestBatchRejectsUnknownAction(t *testing.T) {
	s, uid := setupSvc(t)
	if _, err := s.Batch(uid, "nuke", keysOf(s, uid), "", nil); err == nil {
		t.Error("未知 action 应报错")
	}
}
