package imagesvc

import (
	"github.com/yixian-huang/imgli/internal/model"
)

// SoftDelete 软删（进回收站，保配额，直链转 410）。非属主→ErrNotFound。
func (s *Service) SoftDelete(userID uint64, key string) error {
	res := s.db.Where("key = ? AND user_id = ?", key, userID).Delete(&model.Image{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// BatchResult 单键批量结果。
type BatchResult struct {
	Key string `json:"key"`
	OK  bool   `json:"ok"`
	Err string `json:"error,omitempty"`
}

// Batch 逐键执行 delete/visibility/move，部分成功。复用单键方法保证归属与校验一致。

// Batch 逐键执行 delete/visibility/move，部分成功。复用单键方法保证归属与校验一致。
func (s *Service) Batch(userID uint64, action string, keys []string, visibility string, albumID *int64) ([]BatchResult, error) {
	switch action {
	case "delete", "visibility", "move":
	default:
		return nil, ErrInvalidAction
	}
	out := make([]BatchResult, 0, len(keys))
	for _, k := range keys {
		var err error
		switch action {
		case "delete":
			err = s.SoftDelete(userID, k)
		case "visibility":
			v := visibility
			_, err = s.Update(userID, k, UpdatePatch{Visibility: &v})
		case "move":
			_, err = s.Update(userID, k, UpdatePatch{AlbumID: albumID})
		}
		br := BatchResult{Key: k, OK: err == nil}
		if err != nil {
			br.Err = err.Error()
		}
		out = append(out, br)
	}
	return out, nil
}

// TrashList 列软删项（deleted_at 倒序，简单 offset-free：按 deleted_at,id keyset 略——回收站量小，用 id 游标）。
