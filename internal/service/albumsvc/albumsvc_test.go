package albumsvc

import (
	"errors"
	"testing"

	"github.com/yixian-huang/imgli/internal/model"
)

func setup(t *testing.T) (*Service, uint64) {
	db := model.TestDB(t)
	u := &model.User{Username: "a", Email: "a@img.li", GroupID: 1}
	db.Create(u)
	return New(db), u.ID
}

func TestCreateAndListWithCountCover(t *testing.T) {
	s, uid := setup(t)
	alb, err := s.Create(uid, "工作", "private")
	if err != nil {
		t.Fatal(err)
	}
	// 放两张图进相册（第二张更晚→cover）
	f := &model.File{Hash: "h", StoragePolicyID: 1, Path: "p", Size: 1, RefCount: 1}
	s.db.Create(f)
	s.db.Create(&model.Image{Key: "img000000001", UserID: &uid, FileID: f.ID, AlbumID: &alb.ID, Name: "one", Ext: "png", Visibility: "public", Status: "normal"})
	s.db.Create(&model.Image{Key: "img000000002", UserID: &uid, FileID: f.ID, AlbumID: &alb.ID, Name: "two", Ext: "png", Visibility: "public", Status: "normal"})
	views, err := s.List(uid)
	if err != nil {
		t.Fatal(err)
	}
	if len(views) != 1 || views[0].Count != 2 {
		t.Fatalf("count 应为 2: %+v", views)
	}
	if views[0].CoverKey != "img000000002" {
		t.Errorf("cover 应为最新图, got %q", views[0].CoverKey)
	}
}

func TestDeleteAlbumOnlyDetachesImages(t *testing.T) {
	s, uid := setup(t)
	alb, _ := s.Create(uid, "x", "private")
	f := &model.File{Hash: "h", StoragePolicyID: 1, Path: "p", Size: 1, RefCount: 1}
	s.db.Create(f)
	s.db.Create(&model.Image{Key: "detach000001", UserID: &uid, FileID: f.ID, AlbumID: &alb.ID, Name: "n", Ext: "png", Visibility: "public", Status: "normal"})
	if err := s.Delete(uid, alb.ID, false); err != nil {
		t.Fatal(err)
	}
	var img model.Image
	s.db.Where("key = ?", "detach000001").First(&img)
	if img.AlbumID != nil {
		t.Error("with_images=false 应把图片移入未分类(album_id=NULL)")
	}
	if img.DeletedAt.Valid {
		t.Error("with_images=false 不应删图")
	}
}

func TestDeleteAlbumWithImagesSoftDeletes(t *testing.T) {
	s, uid := setup(t)
	alb, _ := s.Create(uid, "x", "private")
	f := &model.File{Hash: "h", StoragePolicyID: 1, Path: "p", Size: 1, RefCount: 1}
	s.db.Create(f)
	s.db.Create(&model.Image{Key: "withimg00001", UserID: &uid, FileID: f.ID, AlbumID: &alb.ID, Name: "n", Ext: "png", Visibility: "public", Status: "normal"})
	if err := s.Delete(uid, alb.ID, true); err != nil {
		t.Fatal(err)
	}
	var cnt int64
	s.db.Model(&model.Image{}).Where("key = ?", "withimg00001").Count(&cnt) // 软删后默认查不到
	if cnt != 0 {
		t.Error("with_images=true 应软删相册内图片")
	}
}

func TestDeleteAlbumClearsAlbumIDOnTrashedImages(t *testing.T) {
	s, uid := setup(t)
	alb, _ := s.Create(uid, "工作", "private")
	f := &model.File{Hash: "h", StoragePolicyID: 1, Path: "p", Size: 1, RefCount: 1}
	s.db.Create(f)
	// 一张 live 图 + 一张已在回收站的图，都属于该相册
	s.db.Create(&model.Image{Key: "livekey00001", UserID: &uid, FileID: f.ID, AlbumID: &alb.ID, Name: "live", Ext: "png", Visibility: "public", Status: "normal"})
	trashed := &model.Image{Key: "trashkey0001", UserID: &uid, FileID: f.ID, AlbumID: &alb.ID, Name: "trashed", Ext: "png", Visibility: "public", Status: "normal"}
	s.db.Create(trashed)
	s.db.Delete(trashed) // 软删：进回收站，但仍带 album_id

	// with_images=false 删除相册
	if err := s.Delete(uid, alb.ID, false); err != nil {
		t.Fatal(err)
	}
	// live 图 album_id 应清空
	var live model.Image
	s.db.Where("key = ?", "livekey00001").First(&live)
	if live.AlbumID != nil {
		t.Errorf("live 图 album_id 应清空, got %v", live.AlbumID)
	}
	// 已在回收站的图 album_id 也应清空(不能悬挂指向已删相册)——用 Unscoped 才查得到
	var tr model.Image
	s.db.Unscoped().Where("key = ?", "trashkey0001").First(&tr)
	if tr.AlbumID != nil {
		t.Errorf("回收站中的图 album_id 也应清空, got %v", tr.AlbumID)
	}
}

func TestAlbumForeignReturnsNotFound(t *testing.T) {
	s, uid := setup(t)
	alb, _ := s.Create(uid, "x", "private")
	other := &model.User{Username: "b", Email: "b@img.li", GroupID: 1}
	s.db.Create(other)
	if _, err := s.Get(other.ID, alb.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("他人相册应 ErrNotFound, got %v", err)
	}
}

func TestUpdateDefaultView(t *testing.T) {
	s, uid := setup(t)
	alb, err := s.Create(uid, "视图", "public")
	if err != nil {
		t.Fatal(err)
	}
	bad := "carousel"
	if _, err := s.Update(uid, alb.ID, UpdatePatch{DefaultView: &bad}); !errors.Is(err, ErrInvalidDefaultView) {
		t.Fatalf("非法 default_view 应 ErrInvalidDefaultView, got %v", err)
	}
	imm := "immersive"
	got, err := s.Update(uid, alb.ID, UpdatePatch{DefaultView: &imm})
	if err != nil {
		t.Fatal(err)
	}
	if got.DefaultView != "immersive" {
		t.Errorf("DefaultView=%q want immersive", got.DefaultView)
	}
	v, err := s.GetPublic(alb.ID)
	if err != nil {
		t.Fatal(err)
	}
	if NormalizeDefaultView(v.Album.DefaultView) != "immersive" {
		t.Errorf("public DefaultView=%q", v.Album.DefaultView)
	}
}

func TestGetPublicOwnerAndVisibility(t *testing.T) {
	s, uid := setup(t)
	// setup 创建的用户无 nickname/public_profile；补全
	s.db.Model(&model.User{}).Where("id = ?", uid).Updates(map[string]any{
		"nickname": "阿狸", "public_profile": true, "status": "active",
	})
	priv, _ := s.Create(uid, "私密", "private")
	if _, err := s.GetPublic(priv.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("私密相册 GetPublic 应 ErrNotFound, got %v", err)
	}
	pub, _ := s.Create(uid, "公开游记", "public")
	f := &model.File{Hash: "hp", StoragePolicyID: 1, Path: "pp", Size: 1, RefCount: 1}
	s.db.Create(f)
	// 仅 public+normal 计入访客
	s.db.Create(&model.Image{Key: "pubimg000001", UserID: &uid, FileID: f.ID, AlbumID: &pub.ID, Name: "ok", Ext: "png", Visibility: "public", Status: "normal"})
	s.db.Create(&model.Image{Key: "privimg00001", UserID: &uid, FileID: f.ID, AlbumID: &pub.ID, Name: "hid", Ext: "png", Visibility: "private", Status: "normal"})

	v, err := s.GetPublic(pub.ID)
	if err != nil {
		t.Fatal(err)
	}
	if v.Count != 1 {
		t.Errorf("访客 count 应为 1（仅公开图）, got %d", v.Count)
	}
	if v.Owner == nil || v.Owner.Username != "a" || v.Owner.Nickname != "阿狸" || !v.Owner.PublicProfile {
		t.Errorf("Owner 异常: %+v", v.Owner)
	}
	items, _, err := s.ListPublicImages(pub.ID, "", 24)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Key != "pubimg000001" {
		t.Errorf("ListPublicImages 应只吐公开图: %+v", items)
	}
}
