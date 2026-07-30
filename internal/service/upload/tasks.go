package upload

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/moderation"
	"github.com/yixian-huang/imgli/internal/service/settings"
)

// enqueueModerate 投递机审任务。是否 enabled 仍在任务执行时判断；此处按
// login_sample_rate 做登录用户抽检（游客全审）。runner 可能为 nil（单测）。
func (s *Service) enqueueModerate(imageID uint64) {
	if s.run == nil {
		return
	}
	// 读图拿 key/user 做抽检；失败则入队（fail-open 到机审侧）
	var img model.Image
	if err := s.db.Select("id", "key", "user_id").First(&img, imageID).Error; err == nil {
		cfg := moderation.DefaultConfig()
		if err := s.st.Get(model.SettingModeration, &cfg); err != nil && !errors.Is(err, settings.ErrNotFound) {
			slog.Warn("enqueueModerate 读配置失败，仍入队", "err", err)
		} else {
			moderation.NormalizeConfig(&cfg)
			isGuest := img.UserID == nil
			if !moderation.ShouldEnqueueModerate(isGuest, cfg.LoginSampleRate, img.Key) {
				slog.Info("机审抽检跳过", "image_id", imageID, "key", img.Key, "rate", cfg.LoginSampleRate)
				return
			}
		}
	}
	payload, _ := json.Marshal(map[string]any{"image_id": imageID})
	if err := s.run.Enqueue("moderate_image", string(payload)); err != nil {
		slog.Error("投递 moderate_image 任务失败", "image_id", imageID, "err", err)
	}
}

func (s *Service) enqueueDelete(policyID uint64, key string) {
	payload, _ := json.Marshal(map[string]any{"policy_id": policyID, "key": key})
	if err := s.run.Enqueue("delete_file", string(payload)); err != nil {
		slog.Error("投递 delete_file 回滚任务失败", "policy_id", policyID, "key", key, "err", err)
	}
}

// DeleteFileTask 删除物理文件（供 task runner 注册）。payload = {"policy_id":N,"key":"..."}。

// DeleteFileTask 删除物理文件（供 task runner 注册）。payload = {"policy_id":N,"key":"..."}。
func (s *Service) DeleteFileTask(ctx context.Context, payload string) error {
	var p struct {
		PolicyID uint64 `json:"policy_id"`
		Key      string `json:"key"`
	}
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		return err
	}
	var policy model.StoragePolicy
	if err := s.db.First(&policy, p.PolicyID).Error; err != nil {
		return err
	}
	d, err := s.res.Driver(&policy)
	if err != nil {
		return err
	}
	return d.Delete(ctx, p.Key)
}
