package storagesvc

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/yixian-huang/imgli/internal/config"
	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/storage"
)

func TestMigrateFilesLocalToLocal(t *testing.T) {
	dir := t.TempDir()
	db := model.TestDB(t)
	// 播种已有 id=1 local；再建目标 local 策略
	to := model.StoragePolicy{
		Name: "local-b", Driver: "local",
		Config: map[string]string{"root": "uploads-b"}, Enabled: true,
		PathTemplate: "{Y}/{m}/{d}/{uniqid}.{ext}",
	}
	if err := db.Create(&to).Error; err != nil {
		t.Fatal(err)
	}
	var from model.StoragePolicy
	if err := db.First(&from, 1).Error; err != nil {
		t.Fatal(err)
	}

	r := New(&config.Config{DataDir: dir}, db)
	src, err := r.Driver(&from)
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("migrate-payload-png-bytes")
	path := "2026/07/20/abc123xyz.png"
	if err := src.Put(context.Background(), path, bytes.NewReader(payload)); err != nil {
		t.Fatal(err)
	}
	// 假缩略图
	hash := "deadbeefcafebabe"
	if err := src.Put(context.Background(), ThumbKey(model.SurfacePublic, hash), bytes.NewReader([]byte("jpgthumb"))); err != nil {
		t.Fatal(err)
	}
	f := model.File{
		Hash: hash, StoragePolicyID: from.ID, Path: path,
		Size: int64(len(payload)), MIME: "image/png", RefCount: 1,
	}
	if err := db.Create(&f).Error; err != nil {
		t.Fatal(err)
	}

	// dry-run
	dry, err := r.MigrateFiles(context.Background(), db, MigrateOpts{
		FromPolicyID: from.ID, ToPolicyID: to.ID, DryRun: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if dry.Scanned != 1 || dry.Copied != 1 {
		t.Fatalf("dry-run = %+v", dry)
	}
	// 库仍指向 from
	var check model.File
	db.First(&check, f.ID)
	if check.StoragePolicyID != from.ID {
		t.Fatal("dry-run 不应改库")
	}

	res, err := r.MigrateFiles(context.Background(), db, MigrateOpts{
		FromPolicyID: from.ID, ToPolicyID: to.ID, DeleteSource: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Copied != 1 || res.Failed != 0 {
		t.Fatalf("migrate = %+v", res)
	}
	db.First(&check, f.ID)
	if check.StoragePolicyID != to.ID {
		t.Fatalf("policy_id=%d want %d", check.StoragePolicyID, to.ID)
	}
	dst, _ := r.Driver(&to)
	rc, err := dst.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(rc)
	rc.Close()
	if !bytes.Equal(got, payload) {
		t.Fatalf("payload mismatch %q", got)
	}
	// 源已删
	if _, err := src.Open(context.Background(), path); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("源应已删, err=%v", err)
	}
	// 缩略图在目标
	if ok, _ := dst.Exists(context.Background(), ThumbKey(model.SurfacePublic, hash)); !ok {
		t.Error("目标应有 jpg 缩略图")
	}
}

func TestMigrateFilesRejectsSamePolicy(t *testing.T) {
	db := model.TestDB(t)
	r := New(&config.Config{DataDir: t.TempDir()}, db)
	_, err := r.MigrateFiles(context.Background(), db, MigrateOpts{FromPolicyID: 1, ToPolicyID: 1})
	if err == nil {
		t.Fatal("same policy should fail")
	}
}

func TestMigrateFilesMissingSourceSkipped(t *testing.T) {
	dir := t.TempDir()
	db := model.TestDB(t)
	to := model.StoragePolicy{
		Name: "b", Driver: "local", Config: map[string]string{"root": "b"}, Enabled: true,
	}
	db.Create(&to)
	f := model.File{
		Hash: "h1", StoragePolicyID: 1, Path: "no/such/file.png",
		Size: 1, MIME: "image/png", RefCount: 1,
	}
	db.Create(&f)
	r := New(&config.Config{DataDir: dir}, db)
	res, err := r.MigrateFiles(context.Background(), db, MigrateOpts{
		FromPolicyID: 1, ToPolicyID: to.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Skipped != 1 || res.Copied != 0 {
		t.Fatalf("want skipped=1, got %+v", res)
	}
}
