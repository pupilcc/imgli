package adminsvc

import (
	"testing"
	"time"

	"github.com/yixian-huang/imgli/internal/model"
)

func TestStatsAndAudit(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)

	u := &model.User{Username: "a", Email: "a@x", GroupID: 1, UsedStorage: 1000}
	if err := db.Create(u).Error; err != nil {
		t.Fatal(err)
	}
	f := &model.File{Hash: "h1", StoragePolicyID: 1, Path: "p", Size: 500, RefCount: 1}
	db.Create(f)
	db.Create(&model.Image{Key: "k1", UserID: &u.ID, FileID: f.ID, Name: "a.png", Ext: "png", Status: "normal", CreatedAt: time.Now()})
	db.Create(&model.Image{Key: "k2", UserID: &u.ID, FileID: f.ID, Name: "b.png", Ext: "png", Status: "pending", CreatedAt: time.Now().AddDate(0, 0, -3)})
	db.Create(&model.Image{Key: "k3", UserID: &u.ID, FileID: f.ID, Name: "c.png", Ext: "png", Status: "rejected", CreatedAt: time.Now()})
	db.Create(&model.Task{Type: "moderate_image", Payload: "{}", Status: "pending", RunAt: time.Now()})
	db.Create(&model.Task{Type: "delete_file", Payload: "{}", Status: "running", RunAt: time.Now()})

	st, err := svc.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if st.Users != 1 || st.Images != 3 || st.Storage != 1000 || st.TodayUploads != 2 {
		t.Errorf("stats = %+v", st)
	}
	if st.PendingImages != 1 || st.RejectedImages != 1 {
		t.Errorf("pending/rejected = %d/%d", st.PendingImages, st.RejectedImages)
	}
	if st.TasksPending != 1 || st.TasksRunning != 1 {
		t.Errorf("tasks = pending %d running %d", st.TasksPending, st.TasksRunning)
	}
	if len(st.Daily) == 0 {
		t.Errorf("daily 为空")
	}

	svc.Audit(&u.ID, "admin", "user_ban", map[string]any{"target": 9}, "1.2.3.4")
	var logs []model.AuditLog
	db.Find(&logs)
	if len(logs) != 1 || logs[0].Action != "user_ban" || logs[0].ActorType != "admin" {
		t.Errorf("audit = %+v", logs)
	}
}
