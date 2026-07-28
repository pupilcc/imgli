package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/http"
	"time"

	"github.com/yixian-huang/imgli/internal/service/moderation"
)

// 1×1 PNG(内置测试图,base64)。测试审核只验凭据/连通,不关心图内容。
const testModerationPNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="

// TestModeration POST /api/v1/admin/settings/moderation/test —— 用已保存配置对内置测试图打分。
func (h *AdminHandlers) TestModeration(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.D.Adm.ModerationConfig()
	if err != nil {
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	if !cfg.Enabled {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "请先启用并保存机审配置")
		return
	}
	// aliyun 按公网 imageURL 回抓,内置测试图无公网 URL→Score 恒降级返 0,
	// 会误报"测试完成"(codex 终审)。明确告知不支持,而非给出虚假成功。
	if cfg.Provider == "aliyun" {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "阿里云按图片 URL 审核,测试审核暂不支持;请上传一张公开图片实际验证")
		return
	}
	png, _ := base64.StdEncoding.DecodeString(testModerationPNGBase64)
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	// 内置测试图无公网 URL,aliyun 会降级跳过(score 0);其余 provider 正常打分。
	score, err := moderation.NewScorerFromConfig(cfg).Score(ctx, bytes.NewReader(png), "image/png", "")
	if err != nil {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "测试失败:"+err.Error())
		return
	}
	actor := PrincipalFrom(r).User
	h.D.Adm.Audit(&actor.ID, "admin", "moderation_test", map[string]any{"provider": cfg.Provider, "score": score}, ClientIP(r))
	OK(w, map[string]any{"score": score})
}
