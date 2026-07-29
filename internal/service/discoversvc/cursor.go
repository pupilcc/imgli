package discoversvc

import (
	"encoding/base64"
	"strconv"
	"strings"
)

// feedCursor 是广场/用户公开列表的 keyset 游标。
// new: Val = created_at.Unix()（秒；与 SQLite 时间比较粒度一致）；hot: Val = 累计 views。
// 编码为 base64url，字段用 \x1f 分隔后整体编码。
type feedCursor struct {
	Sort string
	Val  int64
	ID   uint64
}

func encodeCursor(c feedCursor) string {
	raw := c.Sort + "\x1f" + strconv.FormatInt(c.Val, 10) + "\x1f" + strconv.FormatUint(c.ID, 10)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(s string) (feedCursor, error) {
	outer, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return feedCursor{}, ErrBadCursor
	}
	parts := strings.SplitN(string(outer), "\x1f", 3)
	if len(parts) != 3 {
		return feedCursor{}, ErrBadCursor
	}
	val, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return feedCursor{}, ErrBadCursor
	}
	id, err := strconv.ParseUint(parts[2], 10, 64)
	if err != nil {
		return feedCursor{}, ErrBadCursor
	}
	return feedCursor{Sort: parts[0], Val: val, ID: id}, nil
}
