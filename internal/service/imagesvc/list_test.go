package imagesvc

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/yixian-huang/imgli/internal/config"
	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/storagesvc"
)

func TestCursorRoundTripDate(t *testing.T) {
	c := listCursor{Sort: "date", ValInt: 1699999999000000, ID: 42}
	s := encodeListCursor(c)
	got, err := decodeListCursor(s)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Sort != "date" || got.ValInt != c.ValInt || got.ID != 42 {
		t.Fatalf("round-trip 不一致: %+v", got)
	}
}

func TestCursorRoundTripName(t *testing.T) {
	c := listCursor{Sort: "name", ValStr: "a_b\x1fc", ID: 7} // 含分隔符也要安全
	got, err := decodeListCursor(encodeListCursor(c))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ValStr != "a_b\x1fc" || got.ID != 7 {
		t.Fatalf("name 游标损坏: %+v", got)
	}
}

func TestCursorRejectsGarbage(t *testing.T) {
	if _, err := decodeListCursor("!!!not-base64!!!"); err == nil {
		t.Error("应拒绝非法游标")
	}
}

// setupSvc 造库、一个用户、两条图片记录（不同 size/name/时间），返回服务。
func setupSvc(t *testing.T) (*Service, uint64) {
	t.Helper()
	db := model.TestDB(t)
	u := &model.User{Username: "u1", Email: "u1@img.li", GroupID: 1}
	if err := db.Create(u).Error; err != nil {
		t.Fatal(err)
	}
	pol := model.StoragePolicy{}
	db.First(&pol, 1) // TestDB 已 seed 默认本地策略 id=1
	cfg := &config.Config{DataDir: t.TempDir()}
	res := storagesvc.New(cfg, db)
	d, _ := res.Driver(&pol)
	mk := func(name, ext string, size int64, vis string, albumID *uint64) {
		path := storagesvc.SurfacePrefix(vis) + "p/" + name // surface==visibility,路径带前缀(S1 一致)
		f := &model.File{Hash: name + "hash", Surface: vis, StoragePolicyID: 1, Path: path,
			Size: size, MIME: "image/" + ext, Width: 10, Height: 10, RefCount: 1}
		db.Create(f)
		_ = d.Put(context.Background(), path, bytes.NewReader([]byte("x"))) // 真对象,供切换复制
		img := &model.Image{Key: name + "key01", UserID: &u.ID, FileID: f.ID, AlbumID: albumID,
			Name: name, Ext: ext, Visibility: vis, Status: "normal"}
		db.Create(img)
	}
	mk("alpha", "png", 300, "public", nil)
	mk("bravo", "jpg", 100, "private", nil)
	return New(db, res, nil), u.ID
}

func TestListDefaultSortNewestFirst(t *testing.T) {
	s, uid := setupSvc(t)
	rows, next, err := s.List(uid, Filter{}, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("应返回 2 条, got %d", len(rows))
	}
	if next != "" {
		t.Errorf("无更多页 next 应为空, got %q", next)
	}
	// bravo 后建 → 最新在前
	if rows[0].Img.Name != "bravo" {
		t.Errorf("默认按时间倒序, got first=%q", rows[0].Img.Name)
	}
	if rows[0].File.Size != 100 {
		t.Errorf("Row 应带 File, got size=%d", rows[0].File.Size)
	}
}

func TestListFilterVisibilityAndFormat(t *testing.T) {
	s, uid := setupSvc(t)
	rows, _, _ := s.List(uid, Filter{Visibility: "private"}, "", 10)
	if len(rows) != 1 || rows[0].Img.Name != "bravo" {
		t.Fatalf("private 过滤失败: %+v", rows)
	}
	rows, _, _ = s.List(uid, Filter{Format: "PNG"}, "", 10)
	if len(rows) != 1 || rows[0].Img.Name != "alpha" {
		t.Fatalf("PNG 过滤失败: %+v", rows)
	}
}

func TestListSortBySizeAndPagination(t *testing.T) {
	s, uid := setupSvc(t)
	// 每页 1 条，按 size 倒序：alpha(300) 先，游标翻页得 bravo(100)
	rows, next, _ := s.List(uid, Filter{Sort: "size"}, "", 1)
	if len(rows) != 1 || rows[0].Img.Name != "alpha" {
		t.Fatalf("size 排序首项应 alpha, got %+v", rows)
	}
	if next == "" {
		t.Fatal("应有 next_cursor")
	}
	rows2, next2, _ := s.List(uid, Filter{Sort: "size"}, next, 1)
	if len(rows2) != 1 || rows2[0].Img.Name != "bravo" {
		t.Fatalf("第二页应 bravo, got %+v", rows2)
	}
	if next2 != "" {
		t.Errorf("末页 next 应空, got %q", next2)
	}
}

func TestListDefaultSortPaginatesAcrossPages(t *testing.T) {
	s, uid := setupSvc(t) // 2 images (alpha, bravo)
	// add a 3rd so limit=1 yields 3 pages
	f3 := &model.File{Hash: "charliehash", StoragePolicyID: 1, Path: "p/charlie", Size: 200, MIME: "image/png", Width: 10, Height: 10, RefCount: 1}
	s.db.Create(f3)
	s.db.Create(&model.Image{Key: "charliekey01", UserID: &uid, FileID: f3.ID, Name: "charlie", Ext: "png", Visibility: "public", Status: "normal"})
	seen := map[string]bool{}
	cursor := ""
	for i := 0; i < 6; i++ { // safety bound > page count
		rows, next, err := s.List(uid, Filter{}, cursor, 1) // default (date) sort, 1 per page
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range rows {
			seen[r.Img.Key] = true
		}
		if next == "" {
			break
		}
		cursor = next
	}
	if len(seen) != 3 {
		t.Fatalf("默认排序游标翻页应遍历全部 3 张(修复前 page2 为空只见 1 张), got %d", len(seen))
	}
}

func TestListRejectsBadSortAndCursor(t *testing.T) {
	s, uid := setupSvc(t)
	if _, _, err := s.List(uid, Filter{Sort: "bogus"}, "", 10); err == nil {
		t.Error("非法 sort 应报错")
	}
	if _, _, err := s.List(uid, Filter{}, "@@bad@@", 10); err == nil {
		t.Error("非法 cursor 应报错")
	}
}

// TestListHidesExpiredImages 已过期图不在 List；未过期/永久图仍出现，且 Row 可取 expires_at。
func TestListHidesExpiredImages(t *testing.T) {
	s, uid := setupSvc(t) // alpha, bravo 永久

	past := time.Now().Add(-time.Hour)
	future := time.Now().Add(24 * time.Hour)
	mk := func(name string, exp *time.Time) {
		f := &model.File{
			Hash: name + "hash", StoragePolicyID: 1, Path: "p/" + name,
			Size: 50, MIME: "image/png", Width: 10, Height: 10, RefCount: 1,
		}
		s.db.Create(f)
		img := &model.Image{
			Key: name + "key01", UserID: &uid, FileID: f.ID,
			Name: name, Ext: "png", Visibility: "public", Status: "normal",
			ExpiresAt: exp,
		}
		s.db.Create(img)
	}
	mk("expired", &past)
	mk("alive", &future)

	rows, _, err := s.List(uid, Filter{}, "", 20)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	var alive *Row
	for i := range rows {
		names[rows[i].Img.Name] = true
		if rows[i].Img.Name == "alive" {
			alive = &rows[i]
		}
	}
	if names["expired"] {
		t.Error("过期图不应出现在 List")
	}
	if !names["alive"] {
		t.Error("未过期图应出现在 List")
	}
	if !names["alpha"] || !names["bravo"] {
		t.Error("永久图应出现在 List")
	}
	if alive == nil || alive.Img.ExpiresAt == nil {
		t.Fatal("未过期图 Row 应带 ExpiresAt")
	}
	// 永久图 expires_at 为 nil
	for _, r := range rows {
		if r.Img.Name == "alpha" && r.Img.ExpiresAt != nil {
			t.Errorf("永久图 alpha ExpiresAt 应 nil, got %v", r.Img.ExpiresAt)
		}
	}

	// TrashList 不过滤过期（任务禁区：不改 TrashList）——仅确认 List 过滤独立
	// 软删一张未过期图进回收站，TrashList 仍可见
	if err := s.db.Where("name = ?", "alive").Delete(&model.Image{}).Error; err != nil {
		t.Fatal(err)
	}
	trash, _, err := s.TrashList(uid, "", 20)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range trash {
		if r.Img.Name == "alive" {
			found = true
		}
	}
	if !found {
		t.Error("TrashList 不应被 expires 过滤影响")
	}
}
