package adminsvc

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/yixian-huang/imgli/internal/model"
)

func TestListLogs(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)

	// Create test audit logs
	now := time.Now()
	adminID := uint64(1)
	logs := []model.AuditLog{
		{
			ActorID:   &adminID,
			ActorType: "admin",
			Action:    "user_update",
			Detail:    `{"target_id": 1}`,
			IP:        "127.0.0.1",
			CreatedAt: now.Add(-2 * time.Hour),
		},
		{
			ActorID:   &adminID,
			ActorType: "admin",
			Action:    "user_reset_password",
			Detail:    `{"target_id": 2}`,
			IP:        "127.0.0.1",
			CreatedAt: now.Add(-1 * time.Hour),
		},
		{
			ActorID:   &adminID,
			ActorType: "admin",
			Action:    "image_admin_delete",
			Detail:    `{"key": "abc123"}`,
			IP:        "127.0.0.1",
			CreatedAt: now.Add(-30 * time.Minute),
		},
		{
			ActorID:   &adminID,
			ActorType: "admin",
			Action:    "group_create",
			Detail:    `{"name": "test"}`,
			IP:        "127.0.0.1",
			CreatedAt: now,
		},
	}

	for _, log := range logs {
		if err := db.Create(&log).Error; err != nil {
			t.Fatalf("failed to create audit log: %v", err)
		}
	}

	t.Run("list all logs descending", func(t *testing.T) {
		rows, total, err := svc.ListLogs("", "", 1, 50)
		if err != nil {
			t.Fatalf("ListLogs failed: %v", err)
		}
		if total != 4 {
			t.Errorf("expected 4 logs, got %d", total)
		}
		if len(rows) != 4 {
			t.Errorf("expected 4 rows, got %d", len(rows))
		}
		// Should be in reverse chronological order (most recent first)
		if rows[0].Action != "group_create" || rows[1].Action != "image_admin_delete" ||
			rows[2].Action != "user_reset_password" || rows[3].Action != "user_update" {
			t.Errorf("logs not in descending CreatedAt order")
		}
	})

	t.Run("filter by action", func(t *testing.T) {
		rows, total, err := svc.ListLogs("user_update", "", 1, 50)
		if err != nil {
			t.Fatalf("ListLogs failed: %v", err)
		}
		if total != 1 {
			t.Errorf("expected 1 log, got %d", total)
		}
		if len(rows) != 1 {
			t.Errorf("expected 1 row, got %d", len(rows))
		}
		if rows[0].Action != "user_update" {
			t.Errorf("expected action 'user_update', got '%s'", rows[0].Action)
		}
	})

	t.Run("filter by actor_type", func(t *testing.T) {
		rows, total, err := svc.ListLogs("", "admin", 1, 50)
		if err != nil {
			t.Fatalf("ListLogs failed: %v", err)
		}
		if total != 4 {
			t.Errorf("expected 4 logs, got %d", total)
		}
		if len(rows) != 4 {
			t.Errorf("expected 4 rows, got %d", len(rows))
		}
	})

	t.Run("filter by action and actor_type", func(t *testing.T) {
		rows, total, err := svc.ListLogs("image_admin_delete", "admin", 1, 50)
		if err != nil {
			t.Fatalf("ListLogs failed: %v", err)
		}
		if total != 1 {
			t.Errorf("expected 1 log, got %d", total)
		}
		if len(rows) != 1 {
			t.Errorf("expected 1 row, got %d", len(rows))
		}
	})

	t.Run("pagination", func(t *testing.T) {
		rows, total, err := svc.ListLogs("", "", 1, 2)
		if err != nil {
			t.Fatalf("ListLogs failed: %v", err)
		}
		if total != 4 {
			t.Errorf("expected 4 total logs, got %d", total)
		}
		if len(rows) != 2 {
			t.Errorf("expected 2 rows on page 1, got %d", len(rows))
		}

		rows, total, err = svc.ListLogs("", "", 2, 2)
		if err != nil {
			t.Fatalf("ListLogs failed: %v", err)
		}
		if total != 4 {
			t.Errorf("expected 4 total logs, got %d", total)
		}
		if len(rows) != 2 {
			t.Errorf("expected 2 rows on page 2, got %d", len(rows))
		}
	})

	t.Run("detail is preserved as json string", func(t *testing.T) {
		rows, _, err := svc.ListLogs("group_create", "", 1, 50)
		if err != nil {
			t.Fatalf("ListLogs failed: %v", err)
		}
		if len(rows) != 1 {
			t.Fatalf("expected 1 row, got %d", len(rows))
		}
		// Should be able to parse detail as JSON
		var detail map[string]interface{}
		err = json.Unmarshal([]byte(rows[0].Detail), &detail)
		if err != nil {
			t.Errorf("detail is not valid JSON: %v", err)
		}
		if detail["name"] != "test" {
			t.Errorf("expected detail.name='test', got %v", detail["name"])
		}
	})

	t.Run("no results returns empty slice", func(t *testing.T) {
		rows, total, err := svc.ListLogs("nonexistent_action", "", 1, 50)
		if err != nil {
			t.Fatalf("ListLogs failed: %v", err)
		}
		if total != 0 {
			t.Errorf("expected 0 total logs, got %d", total)
		}
		if len(rows) != 0 {
			t.Errorf("expected 0 rows, got %d", len(rows))
		}
	})
}
