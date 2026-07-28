package s3

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ErrPresignUnconfigured 策略未配 presign_domain——调用方应回落流式,不是错误路径。
var ErrPresignUnconfigured = errors.New("s3: 未配置 presign_domain")

// s3QueryEncode 与 s3URIEncode 同规则,但 '/' 也编码。query 参数值里的 '/' 必须
// 是 %2F(X-Amz-Credential 的 scope 分隔符),否则签名串与服务端重算的不一致。
func s3QueryEncode(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9',
			c == '-', c == '.', c == '_', c == '~':
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

// PresignGet 签出 SigV4 query-string 预签名 GET URL,指向 presign_domain。
//
// 恒用 path-style(/{bucket}/{prefix+key}):presign_domain 是我们自建的直连入口,
// 不做 virtual-host 桶名解析,也不需要通配 DNS。
//
// 2026-07-23 已对 RustFS 真机验证:有效签名 200、篡改 403、过期 403
// (internal/storage/s3/presign_live_test.go)。
func (d *Driver) PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	if d.presignHost == "" {
		return "", ErrPresignUnconfigured
	}
	// 与 do() 的 d.prefix+objectKey 保持一致,否则签出的键与实际对象对不上。
	uri := "/" + d.bucket + "/" + s3URIEncode(d.prefix+key)

	now := d.nowOr().UTC()
	amzDate := now.Format("20060102T150405Z")
	date := now.Format("20060102")
	scope := d.accessKey + "/" + date + "/" + d.region + "/s3/aws4_request"

	// canonicalQuery 必须按参数名字典序,值按 RFC3986 编码。
	canonicalQuery := strings.Join([]string{
		"X-Amz-Algorithm=AWS4-HMAC-SHA256",
		"X-Amz-Credential=" + s3QueryEncode(scope),
		"X-Amz-Date=" + amzDate,
		"X-Amz-Expires=" + strconv.Itoa(int(ttl.Seconds())),
		"X-Amz-SignedHeaders=host",
	}, "&")

	sig, _ := SignV4Raw("GET", uri, canonicalQuery, d.secretKey, d.region, "s3",
		amzDate, date, "UNSIGNED-PAYLOAD", map[string]string{"host": d.presignHost})

	return d.presignScheme + "://" + d.presignHost + uri + "?" + canonicalQuery +
		"&X-Amz-Signature=" + sig, nil
}
