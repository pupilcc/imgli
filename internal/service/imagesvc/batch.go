package imagesvc

import (
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

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
	Key     string `json:"key"`
	OK      bool   `json:"ok"`
	Skipped bool   `json:"skipped,omitempty"` // rename：新名与旧名相同
	Err     string `json:"error,omitempty"`
}

// BatchOpts 批量操作可选参数。
type BatchOpts struct {
	Visibility  string
	AlbumID     *int64
	// rename：查找替换（可选）→ 模板（可选）；至少其一
	// 模板字面量可任意写；占位：{name}{original}{ext}{n}{n:03}{yyyy}{mm}{dd}{album}；{{ → {
	NamePattern       string
	Find              string // 空=不查找；| 或换行多关键字
	Replace           string
	ReplaceIgnoreCase bool
	CleanSeparators   bool
	StartN            int    // 序号起点，默认 1
	AlbumName         string // {album}；相册页批量时由前端传入
}

// Batch 逐键执行 delete|visibility|move|rename，部分成功。
func (s *Service) Batch(userID uint64, action string, keys []string, opts BatchOpts) ([]BatchResult, error) {
	switch action {
	case "delete", "visibility", "move", "rename":
	default:
		return nil, ErrInvalidAction
	}
	if action == "rename" {
		return s.batchRename(userID, keys, opts)
	}
	out := make([]BatchResult, 0, len(keys))
	for _, k := range keys {
		var err error
		switch action {
		case "delete":
			err = s.SoftDelete(userID, k)
		case "visibility":
			v := opts.Visibility
			_, err = s.Update(userID, k, UpdatePatch{Visibility: &v})
		case "move":
			_, err = s.Update(userID, k, UpdatePatch{AlbumID: opts.AlbumID})
		}
		br := BatchResult{Key: k, OK: err == nil}
		if err != nil {
			br.Err = err.Error()
		}
		out = append(out, br)
	}
	return out, nil
}

func imageBaseName(name, ext string) string {
	base := strings.TrimSuffix(name, path.Ext(name))
	if base == "" {
		base = strings.TrimSuffix(name, "."+ext)
	}
	return base
}

func ensureExt(name, ext string) string {
	if ext == "" {
		return name
	}
	if strings.HasSuffix(strings.ToLower(name), "."+strings.ToLower(ext)) {
		return name
	}
	return name + "." + ext
}

type renamePlan struct {
	key, oldName, newName string
}

func (s *Service) batchRename(userID uint64, keys []string, opts BatchOpts) ([]BatchResult, error) {
	find := strings.TrimSpace(opts.Find)
	pattern := strings.TrimSpace(opts.NamePattern)
	if find == "" && pattern == "" {
		return nil, ErrInvalidName
	}
	start := opts.StartN
	if start < 1 {
		start = 1
	}

	plans := make([]renamePlan, 0, len(keys))
	out := make([]BatchResult, 0, len(keys))

	for i, k := range keys {
		var img model.Image
		if err := s.db.Where("key = ? AND user_id = ?", k, userID).First(&img).Error; err != nil {
			out = append(out, BatchResult{Key: k, OK: false, Err: ErrNotFound.Error()})
			continue
		}
		newName, err := computeRename(img, opts, start+i)
		if err != nil {
			out = append(out, BatchResult{Key: k, OK: false, Err: err.Error()})
			continue
		}
		plans = append(plans, renamePlan{key: k, oldName: img.Name, newName: newName})
	}

	// 本批内目标名冲突（同名）
	counts := make(map[string]int, len(plans))
	for _, p := range plans {
		counts[p.newName]++
	}

	// plans 与 out 可能错位（有些 key 已 fail）；按 key 建 map 再输出顺序跟随 keys
	byKey := make(map[string]renamePlan, len(plans))
	for _, p := range plans {
		byKey[p.key] = p
	}
	// 重建 out 保持 keys 顺序：先处理已失败的
	failed := make(map[string]BatchResult)
	for _, br := range out {
		failed[br.Key] = br
	}
	out = out[:0]
	for _, k := range keys {
		if br, ok := failed[k]; ok {
			out = append(out, br)
			continue
		}
		p, ok := byKey[k]
		if !ok {
			out = append(out, BatchResult{Key: k, OK: false, Err: ErrNotFound.Error()})
			continue
		}
		if p.newName == p.oldName {
			out = append(out, BatchResult{Key: k, OK: true, Skipped: true})
			continue
		}
		if counts[p.newName] > 1 {
			out = append(out, BatchResult{Key: k, OK: false, Err: "rename conflict: duplicate target name"})
			continue
		}
		name := p.newName
		_, err := s.Update(userID, k, UpdatePatch{Name: &name})
		br := BatchResult{Key: k, OK: err == nil}
		if err != nil {
			br.Err = err.Error()
		}
		out = append(out, br)
	}
	return out, nil
}

// computeRename：可选查找替换 → 可选模板。
func computeRename(img model.Image, opts BatchOpts, n1 int) (string, error) {
	original := imageBaseName(img.Name, img.Ext)
	base := original
	find := strings.TrimSpace(opts.Find)
	pattern := strings.TrimSpace(opts.NamePattern)

	if find != "" {
		base = applyFindReplace(base, find, opts.Replace, opts.ReplaceIgnoreCase)
		if opts.CleanSeparators {
			base = cleanNameSeparators(base)
		}
	}
	if pattern != "" {
		ctx := patternCtx{
			name:     base,
			original: original,
			ext:      img.Ext,
			n1:       n1,
			created:  img.CreatedAt,
			album:    strings.TrimSpace(opts.AlbumName),
		}
		base = applyPattern(pattern, ctx)
		if opts.CleanSeparators {
			base = cleanNameSeparators(base)
		}
	}

	base = strings.TrimSpace(base)
	base = sanitizeFileBase(base)
	if base == "" {
		return "", ErrInvalidName
	}
	return ensureExt(base, img.Ext), nil
}

type patternCtx struct {
	name, original, ext, album string
	n1                         int
	created                    time.Time
}

var nPadRe = regexp.MustCompile(`\{n(?::(\d+))?\}`)

func applyPattern(pattern string, ctx patternCtx) string {
	// {{ → 临时标记，避免被占位符吃掉
	const esc = "\x00BRACE\x00"
	out := strings.ReplaceAll(pattern, "{{", esc)
	out = strings.ReplaceAll(out, "{name}", ctx.name)
	out = strings.ReplaceAll(out, "{original}", ctx.original)
	out = strings.ReplaceAll(out, "{ext}", strings.TrimPrefix(ctx.ext, "."))
	out = strings.ReplaceAll(out, "{album}", ctx.album)
	y, m, d := ctx.created.Date()
	if ctx.created.IsZero() {
		now := time.Now()
		y, m, d = now.Date()
	}
	out = strings.ReplaceAll(out, "{yyyy}", fmt.Sprintf("%04d", y))
	out = strings.ReplaceAll(out, "{mm}", fmt.Sprintf("%02d", int(m)))
	out = strings.ReplaceAll(out, "{dd}", fmt.Sprintf("%02d", d))
	out = nPadRe.ReplaceAllStringFunc(out, func(m string) string {
		sub := nPadRe.FindStringSubmatch(m)
		if len(sub) == 2 && sub[1] != "" {
			w, err := strconv.Atoi(sub[1])
			if err == nil && w > 0 && w <= 8 {
				return fmt.Sprintf("%0*d", w, ctx.n1)
			}
		}
		return strconv.Itoa(ctx.n1)
	})
	out = strings.ReplaceAll(out, esc, "{")
	// 末尾若用户写了 .ext 与强制保留扩展冲突时去掉，由 ensureExt 统一加
	out = strings.TrimSuffix(out, path.Ext(out))
	return out
}

func applyFindReplace(base, find, replace string, ignoreCase bool) string {
	for _, term := range splitFindTerms(find) {
		if term == "" {
			continue
		}
		base = replaceAllLiteral(base, term, replace, ignoreCase)
	}
	return base
}

func splitFindTerms(find string) []string {
	raw := strings.ReplaceAll(find, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	var terms []string
	for _, line := range strings.Split(raw, "\n") {
		for _, p := range strings.Split(line, "|") {
			p = strings.TrimSpace(p)
			if p != "" {
				terms = append(terms, p)
			}
		}
	}
	return terms
}

func replaceAllLiteral(s, old, new string, ignoreCase bool) string {
	if old == "" {
		return s
	}
	if !ignoreCase {
		return strings.ReplaceAll(s, old, new)
	}
	lo := strings.ToLower(old)
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	lower := strings.ToLower(s)
	for i < len(s) {
		if i+len(old) <= len(s) && lower[i:i+len(old)] == lo && strings.EqualFold(s[i:i+len(old)], old) {
			b.WriteString(new)
			i += len(old)
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func cleanNameSeparators(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prevSep := false
	for _, r := range s {
		sep := r == ' ' || r == '_' || r == '-'
		if sep {
			if b.Len() == 0 {
				continue
			}
			if prevSep {
				continue
			}
			prevSep = true
			if r == ' ' {
				b.WriteByte('_')
			} else {
				b.WriteRune(r)
			}
			continue
		}
		prevSep = false
		b.WriteRune(r)
	}
	return strings.Trim(b.String(), " _-")
}

func sanitizeFileBase(s string) string {
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "\\", "_")
	s = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 || unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
	return strings.TrimSpace(s)
}
