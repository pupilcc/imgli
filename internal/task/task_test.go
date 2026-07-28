package task

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yixian-huang/imgli/internal/model"
)

func TestEnqueueAndDrainSuccess(t *testing.T) {
	db := model.TestDB(t)
	r := New(db, 2)

	var mu sync.Mutex
	got := []string{}
	r.Register("echo", func(ctx context.Context, payload string) error {
		mu.Lock()
		got = append(got, payload)
		mu.Unlock()
		return nil
	})

	for _, p := range []string{"a", "b", "c"} {
		if err := r.Enqueue("echo", p); err != nil {
			t.Fatal(err)
		}
	}
	n := r.drainOnce(context.Background())
	if n != 3 {
		t.Fatalf("drain 处理 %d 条, want 3", n)
	}
	mu.Lock()
	if len(got) != 3 {
		t.Errorf("处理内容 %v", got)
	}
	mu.Unlock()

	var pending int64
	db.Model(&model.Task{}).Where("status = ?", "pending").Count(&pending)
	if pending != 0 {
		t.Errorf("完成后不应有 pending, got %d", pending)
	}
	var done int64
	db.Model(&model.Task{}).Where("status = ?", "done").Count(&done)
	if done != 3 {
		t.Errorf("done=%d, want 3", done)
	}
}

func TestFailedTaskRetriesThenGivesUp(t *testing.T) {
	db := model.TestDB(t)
	r := New(db, 1)
	r.Register("boom", func(ctx context.Context, payload string) error {
		return errors.New("always fails")
	})
	r.Enqueue("boom", "x")

	// 反复 drain 直到达到重试上限
	for i := 0; i < 10; i++ {
		if r.drainOnce(context.Background()) == 0 {
			break
		}
	}
	var task model.Task
	db.First(&task, "type = ?", "boom")
	if task.Status != "failed" {
		t.Errorf("超过重试上限应为 failed, got %q (attempts=%d)", task.Status, task.Attempts)
	}
	if task.LastError == "" {
		t.Error("failed 任务应记录 last_error")
	}
}

func TestCleanupExpiredSessions(t *testing.T) {
	db := model.TestDB(t)
	r := New(db, 1)
	// sessions.user_id 现有 DB 级 FK，须先建真实 user（复用 TestDB 播种的默认组）。
	var group model.UserGroup
	if err := db.Where("is_default = ?", true).First(&group).Error; err != nil {
		t.Fatal(err)
	}
	u := model.User{Username: "sesu", Email: "sesu@x.li", GroupID: group.ID}
	if err := db.Create(&u).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Session{ID: "live", UserID: u.ID, ExpiresAt: time.Now().Add(time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Session{ID: "dead", UserID: u.ID, ExpiresAt: time.Now().Add(-time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}

	r.cleanupExpired()

	var n int64
	db.Model(&model.Session{}).Count(&n)
	if n != 1 {
		t.Errorf("过期 session 应被清理, 剩 %d", n)
	}
}

func TestHandlerPanicIsRetriedThenFailed(t *testing.T) {
	db := model.TestDB(t)
	r := New(db, 1)
	r.Register("panic", func(ctx context.Context, payload string) error {
		panic("boom")
	})
	if err := r.Enqueue("panic", "x"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		if r.drainOnce(context.Background()) == 0 {
			break
		}
	}
	var task model.Task
	if err := db.First(&task, "type = ?", "panic").Error; err != nil {
		t.Fatal(err)
	}
	if task.Status != "failed" {
		t.Fatalf("panic 达上限应为 failed, got %q attempts=%d err=%q", task.Status, task.Attempts, task.LastError)
	}
	if task.Attempts < maxAttempts {
		t.Fatalf("attempts=%d want >= %d", task.Attempts, maxAttempts)
	}
	if task.LastError == "" || !strings.Contains(task.LastError, "panic") {
		t.Fatalf("last_error 应含 panic, got %q", task.LastError)
	}
}

func TestRequeueAllRunningOnBoot(t *testing.T) {
	db := model.TestDB(t)
	r := New(db, 1)
	old := time.Now().Add(-time.Hour)
	t1 := model.Task{Type: "echo", Payload: "a", Status: "running", RunAt: old, UpdatedAt: old}
	t2 := model.Task{Type: "echo", Payload: "b", Status: "pending", RunAt: old}
	if err := db.Create(&t1).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&t2).Error; err != nil {
		t.Fatal(err)
	}
	r.requeueAllRunning()
	var got model.Task
	db.First(&got, t1.ID)
	if got.Status != "pending" {
		t.Fatalf("boot requeue status=%q want pending", got.Status)
	}
	if got.LastError != "requeued: process restart" {
		t.Fatalf("last_error=%q", got.LastError)
	}
	// 原本 pending 不受影响
	db.First(&got, t2.ID)
	if got.Status != "pending" {
		t.Fatalf("pending 被误改: %q", got.Status)
	}
}

func TestRequeueStuckRunningByAge(t *testing.T) {
	db := model.TestDB(t)
	r := New(db, 1)
	stale := time.Now().Add(-30 * time.Minute)
	fresh := time.Now().Add(-time.Minute)
	old := model.Task{Type: "echo", Payload: "stale-payload", Status: "running", RunAt: stale}
	newT := model.Task{Type: "echo", Payload: "fresh-payload", Status: "running", RunAt: fresh}
	if err := db.Create(&old).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&newT).Error; err != nil {
		t.Fatal(err)
	}
	// Create 会刷 UpdatedAt=now，用 UpdateColumn 钉死年龄
	if err := db.Model(&model.Task{}).Where("id = ?", old.ID).UpdateColumn("updated_at", stale).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.Task{}).Where("id = ?", newT.ID).UpdateColumn("updated_at", fresh).Error; err != nil {
		t.Fatal(err)
	}
	cutoff := time.Now().Add(-stuckRunningAfter)
	r.requeueStuckRunning(cutoff)
	var staleRow, freshRow model.Task
	if err := db.Where("payload = ?", "stale-payload").First(&staleRow).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Where("payload = ?", "fresh-payload").First(&freshRow).Error; err != nil {
		t.Fatal(err)
	}
	if staleRow.Status != "pending" {
		t.Fatalf("stale running 应 requeue, got %q (updated_at=%v cutoff=%v)", staleRow.Status, staleRow.UpdatedAt, cutoff)
	}
	if freshRow.Status != "running" {
		t.Fatalf("新鲜 running 不应 requeue, got %q (updated_at=%v cutoff=%v)", freshRow.Status, freshRow.UpdatedAt, cutoff)
	}
}

func TestConcurrentWorkersNoDoubleExecute(t *testing.T) {
	db := model.TestDB(t)
	r := New(db, 4)
	var mu sync.Mutex
	seen := map[string]int{}
	r.Register("once", func(ctx context.Context, payload string) error {
		mu.Lock()
		seen[payload]++
		mu.Unlock()
		return nil
	})
	for i := 0; i < 20; i++ {
		r.Enqueue("once", "job"+strconv.Itoa(i))
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { r.Start(ctx); close(done) }()
	// 轮询直到全部完成
	deadline := 0
	for {
		var pending int64
		db.Model(&model.Task{}).Where("status = ?", "pending").Count(&pending)
		var running int64
		db.Model(&model.Task{}).Where("status = ?", "running").Count(&running)
		if pending == 0 && running == 0 {
			break
		}
		deadline++
		if deadline > 200 {
			t.Fatal("任务未在预期内完成")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-done
	mu.Lock()
	defer mu.Unlock()
	if len(seen) != 20 {
		t.Fatalf("处理了 %d 个不同任务, want 20", len(seen))
	}
	for p, c := range seen {
		if c != 1 {
			t.Errorf("%s 执行了 %d 次, want 1（双认领！）", p, c)
		}
	}
}
