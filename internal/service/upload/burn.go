package upload

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/yixian-huang/imgli/internal/imaging"
	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/settings"
)

// burn 就地处理临时文件(缩放→站点文字水印→用户图片水印),返回是否发生改写。
// 仅 jpg/jpeg/png。settings 仅「键不存在」按默认降级,其余读取错误上抛中止——
// DB 故障静默全关会绕过已启用的站点水印策略(codex 评审 Task4)。
// 处理链任一步 ErrUnsupported(内容与扩展名不符等)→ 整链放弃、原字节不动、
// 记 slog.Warn:不写回部分处理的中间结果(原子降级语义,codex 评审 Task4)。
func (s *Service) burn(tmpPath, ext string, u *model.User) (bool, error) {
	switch strings.ToLower(ext) {
	case "jpg", "jpeg", "png":
	default:
		return false, nil
	}
	var proc Processing
	if err := s.st.Get(model.SettingProcessing, &proc); err != nil {
		if !errors.Is(err, settings.ErrNotFound) {
			return false, err
		}
		proc = DefaultProcessing()
	}
	// 用户图片水印可用性:登录用户 + 偏好开启 + 目录已装配 + 文件存在
	var markData []byte
	if u != nil && u.Preferences.Watermark.Enabled && s.WatermarkDir != "" {
		if b, err := os.ReadFile(filepath.Join(s.WatermarkDir, fmt.Sprintf("%d.png", u.ID))); err == nil {
			markData = b
		}
	}
	textOn := proc.TextWatermark.Enabled && strings.TrimSpace(proc.TextWatermark.Text) != ""
	stripOn := proc.StripExifEnabled()
	if proc.MaxEdge == 0 && !textOn && markData == nil && !stripOn {
		return false, nil // 无处理项:字节完全不动
	}

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return false, err
	}
	out := data
	// EXIF 剥离放在缩放/水印前：先去元数据再处理；仅剥离子时也走重编码路径。
	if stripOn {
		v, err := imaging.StripMetadata(out)
		if errors.Is(err, imaging.ErrUnsupported) {
			slog.Warn("处理管线整链跳过(内容不可解码)", "step", "strip_exif")
			return false, nil
		}
		if err != nil {
			return false, err
		}
		out = v
	}
	if proc.MaxEdge > 0 {
		v, err := imaging.Scale(out, proc.MaxEdge)
		if errors.Is(err, imaging.ErrUnsupported) {
			slog.Warn("处理管线整链跳过(内容不可解码)", "step", "scale")
			return false, nil
		}
		if err != nil {
			return false, err
		}
		out = v
	}
	if textOn {
		v, err := imaging.WatermarkText(out, proc.TextWatermark.Text,
			proc.TextWatermark.Position, proc.TextWatermark.Opacity, proc.TextWatermark.SizeRatio)
		if errors.Is(err, imaging.ErrUnsupported) {
			slog.Warn("处理管线整链跳过(内容不可解码)", "step", "text_watermark")
			return false, nil
		}
		if err != nil {
			return false, err
		}
		out = v
	}
	if markData != nil {
		w := u.Preferences.Watermark
		pos := w.Position
		if pos == "" {
			pos = "br"
		}
		op := w.Opacity
		if op == 0 {
			op = 0.5
		}
		v, err := imaging.WatermarkImage(out, markData, pos, op, w.Margin)
		if errors.Is(err, imaging.ErrUnsupported) {
			// mark 文件坏/非 PNG:仅此步跳过不弃整链——底图处理(缩放/文字印)已确定可解码,
			// 弃链会连站点策略一起丢
			slog.Warn("处理管线跳过用户水印(水印图不可用)", "user_id", u.ID)
		} else if err != nil {
			return false, err
		} else {
			out = v
		}
	}
	if bytes.Equal(out, data) {
		return false, nil
	}
	// 覆写同一 tmpPath(0600 与 CreateTemp 缺省一致)
	if err := os.WriteFile(tmpPath, out, 0o600); err != nil {
		return false, err
	}
	return true, nil
}

// enqueueModerate 投递机审任务。是否 enabled 仍在任务执行时判断；此处按
// login_sample_rate 做登录用户抽检（游客全审）。runner 可能为 nil（单测）。
