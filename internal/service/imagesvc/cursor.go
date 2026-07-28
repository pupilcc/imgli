package imagesvc

import (
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
)

// listCursor 是列表 keyset 游标：按 sort 决定用 ValInt(date/size) 还是 ValStr(name)，
// 附 ID 作为稳定 tiebreak。编码为 base64url，字段用 \x1f 分隔，值再各自 base64 以容纳任意字节。
type listCursor struct {
	Sort   string
	ValInt int64
	ValStr string
	ID     uint64
}

func encodeListCursor(c listCursor) string {
	// val 统一转字符串（date/size 用十进制），再整体 base64 避免分隔符冲突
	val := c.ValStr
	if c.Sort != "name" {
		val = strconv.FormatInt(c.ValInt, 10)
	}
	raw := c.Sort + "\x1f" + base64.RawURLEncoding.EncodeToString([]byte(val)) +
		"\x1f" + strconv.FormatUint(c.ID, 10)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeListCursor(s string) (listCursor, error) {
	bad := errors.New("游标格式错误")
	outer, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return listCursor{}, bad
	}
	parts := strings.SplitN(string(outer), "\x1f", 3)
	if len(parts) != 3 {
		return listCursor{}, bad
	}
	valBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return listCursor{}, bad
	}
	id, err := strconv.ParseUint(parts[2], 10, 64)
	if err != nil {
		return listCursor{}, bad
	}
	c := listCursor{Sort: parts[0], ID: id}
	if c.Sort == "name" {
		c.ValStr = string(valBytes)
	} else {
		c.ValInt, err = strconv.ParseInt(string(valBytes), 10, 64)
		if err != nil {
			return listCursor{}, bad
		}
	}
	return c, nil
}
