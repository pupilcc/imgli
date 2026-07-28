// Package task 数据库任务表 + 进程内 worker 池 + 定时清理。
// 认领：Postgres 用 FOR UPDATE SKIP LOCKED；SQLite 单写者用普通事务 UPDATE。
package task

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/yixian-huang/imgli/internal/model"
)

const (
	maxAttempts = 3
	// stuckRunningAfter 进程内 hung 的 running 任务超过此时长则重新入队。
	// 进程崩溃后启动时会立即 requeue 全部 running，不依赖本阈值。
	stuckRunningAfter = 15 * time.Minute
)

type HandlerFunc func(ctx context.Context, payload string) error

type Runner struct {
	db       *gorm.DB
	workers  int
	handlers map[string]HandlerFunc
	wake     chan struct{}
}

func New(db *gorm.DB, workers int) *Runner {
	if workers < 1 {
		workers = 1
	}
	return &Runner{
		db: db, workers: workers,
		handlers: map[string]HandlerFunc{},
		wake:     make(chan struct{}, 1),
	}
}

func (r *Runner) Register(taskType string, h HandlerFunc) { r.handlers[taskType] = h }

// Enqueue 落一条 pending 任务并唤醒 worker。
func (r *Runner) Enqueue(taskType, payload string) error {
	t := &model.Task{Type: taskType, Payload: payload, Status: "pending", RunAt: time.Now()}
	if err := r.db.Create(t).Error; err != nil {
		return err
	}
	select {
	case r.wake <- struct{}{}:
	default:
	}
	return nil
}

// claim 认领一条可执行任务并置为 running；无任务返回 (nil,false)。
func (r *Runner) claim() (*model.Task, bool) {
	var t model.Task
	err := r.db.Transaction(func(tx *gorm.DB) error {
		q := tx.Where("status = ? AND run_at <= ?", "pending", time.Now()).Order("id")
		if tx.Dialector.Name() == "postgres" {
			q = q.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
		}
		if err := q.First(&t).Error; err != nil {
			return err
		}
		// 认领：带 status 守卫 + RowsAffected 检测竞态，双方言都安全
		// map Updates 不自动刷新 UpdatedAt，显式写入供 stuck 恢复判定。
		now := time.Now()
		res := tx.Model(&model.Task{}).
			Where("id = ? AND status = ?", t.ID, "pending").
			Updates(map[string]any{"status": "running", "updated_at": now})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound // 被别的 worker 抢先，视为无任务
		}
		return nil
	})
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			slog.Warn("任务认领失败", "err", err)
		}
		return nil, false
	}
	t.Status = "running"
	return &t, true
}

func (r *Runner) run(ctx context.Context, t *model.Task) {
	h, ok := r.handlers[t.Type]
	if !ok {
		r.db.Model(t).Updates(map[string]any{"status": "failed", "last_error": "no handler"})
		return
	}
	var err error
	func() {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("任务 handler panic",
					"type", t.Type, "id", t.ID, "err", rec,
					"stack", string(debug.Stack()))
				err = fmt.Errorf("panic: %v", rec)
			}
		}()
		err = h(ctx, t.Payload)
	}()
	if err == nil {
		r.db.Model(t).Update("status", "done")
		return
	}
	attempts := t.Attempts + 1
	upd := map[string]any{"attempts": attempts, "last_error": err.Error()}
	if attempts >= maxAttempts {
		upd["status"] = "failed"
	} else {
		upd["status"] = "pending"
		upd["run_at"] = time.Now() // 立即重试（受 maxAttempts 上限约束，不做退避以配合同步 drain 测试）
	}
	r.db.Model(t).Updates(upd)
}

// drainOnce 串行认领并执行直到无可执行任务，返回处理条数（测试用；生产用 Start）。
func (r *Runner) drainOnce(ctx context.Context) int {
	n := 0
	for {
		select {
		case <-ctx.Done():
			return n
		default:
		}
		t, ok := r.claim()
		if !ok {
			return n
		}
		r.run(ctx, t)
		n++
	}
}

func (r *Runner) cleanupExpired() {
	now := time.Now()
	r.db.Where("expires_at < ?", now).Delete(&model.Session{})
	r.db.Where("expires_at < ?", now).Delete(&model.AuthToken{})
	r.requeueStuckRunning(now.Add(-stuckRunningAfter))
}

// requeueAllRunning 进程启动时调用：上一轮崩溃遗留的 running 全部重回 pending。
func (r *Runner) requeueAllRunning() {
	res := r.db.Model(&model.Task{}).
		Where("status = ?", "running").
		Updates(map[string]any{
			"status":     "pending",
			"run_at":     time.Now(),
			"last_error": "requeued: process restart",
			"updated_at": time.Now(),
		})
	if res.Error != nil {
		slog.Warn("启动 requeue running 失败", "err", res.Error)
		return
	}
	if res.RowsAffected > 0 {
		slog.Warn("启动 requeue 遗留 running 任务", "n", res.RowsAffected)
	}
}

// requeueStuckRunning 将 updated_at 早于 cutoff 的 running 重新入队（worker hung）。
func (r *Runner) requeueStuckRunning(cutoff time.Time) {
	res := r.db.Model(&model.Task{}).
		Where("status = ? AND updated_at < ?", "running", cutoff).
		Updates(map[string]any{
			"status":     "pending",
			"run_at":     time.Now(),
			"last_error": "requeued: stuck running",
			"updated_at": time.Now(),
		})
	if res.Error != nil {
		slog.Warn("requeue stuck running 失败", "err", res.Error)
		return
	}
	if res.RowsAffected > 0 {
		slog.Warn("requeue 卡死 running 任务", "n", res.RowsAffected)
	}
}

// Start 启动 worker 池与定时清理，阻塞式后台运行至 ctx 取消。
func (r *Runner) Start(ctx context.Context) {
	r.requeueAllRunning()
	var wg sync.WaitGroup
	for i := 0; i < r.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ticker := time.NewTicker(30 * time.Second) // 轮询兜底
			defer ticker.Stop()
			for {
				r.drainOnce(ctx)
				select {
				case <-ctx.Done():
					return
				case <-r.wake:
				case <-ticker.C:
				}
			}
		}()
	}
	go func() {
		ct := time.NewTicker(10 * time.Minute)
		defer ct.Stop()
		r.cleanupExpired()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ct.C:
				r.cleanupExpired()
			}
		}
	}()
	<-ctx.Done()
	wg.Wait()
}
