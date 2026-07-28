package s3

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"
)

func presignDriver(t *testing.T, presignDomain string) *Driver {
	t.Helper()
	d, err := New(map[string]string{
		"endpoint": "192.0.2.10:9000", "region": "us-east-1", "bucket": "imgli",
		"access_key_id": "AKID", "secret_access_key": "SECRET",
		"path_style": "true", "prefix": "prod/", "presign_domain": presignDomain,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	d.now = func() time.Time { return time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC) }
	return d
}

func TestPresignGetURLShape(t *testing.T) {
	d := presignDriver(t, "https://s3.img.li")
	raw, err := d.PresignGet(context.Background(), "2026/07/x y.png", 60*time.Second)
	if err != nil {
		t.Fatalf("PresignGet: %v", err)
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("产出的不是合法 URL: %v", err)
	}
	if u.Scheme != "https" || u.Host != "s3.img.li" {
		t.Errorf("签名目标应为 presign_domain,得到 %s://%s", u.Scheme, u.Host)
	}
	// 恒 path-style,且 prefix 必须参与(与 do() 的 d.prefix+key 一致);空格编码为 %20
	if u.EscapedPath() != "/imgli/prod/2026/07/x%20y.png" {
		t.Errorf("path=%q", u.EscapedPath())
	}
	q := u.Query()
	if q.Get("X-Amz-Algorithm") != "AWS4-HMAC-SHA256" {
		t.Errorf("Algorithm=%q", q.Get("X-Amz-Algorithm"))
	}
	if q.Get("X-Amz-Credential") != "AKID/20260723/us-east-1/s3/aws4_request" {
		t.Errorf("Credential=%q", q.Get("X-Amz-Credential"))
	}
	if q.Get("X-Amz-Date") != "20260723T120000Z" {
		t.Errorf("Date=%q", q.Get("X-Amz-Date"))
	}
	if q.Get("X-Amz-Expires") != "60" {
		t.Errorf("Expires=%q", q.Get("X-Amz-Expires"))
	}
	if q.Get("X-Amz-SignedHeaders") != "host" {
		t.Errorf("SignedHeaders=%q", q.Get("X-Amz-SignedHeaders"))
	}
	if len(q.Get("X-Amz-Signature")) != 64 {
		t.Errorf("Signature 应为 64 位十六进制,得到 %q", q.Get("X-Amz-Signature"))
	}
}

// TestPresignGetDeterministic 固定时钟下签名必须稳定——防止 canonical query
// 顺序或编码被无意改动。
func TestPresignGetDeterministic(t *testing.T) {
	a, err := presignDriver(t, "https://s3.img.li").PresignGet(context.Background(), "k.png", 60*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	b, err := presignDriver(t, "https://s3.img.li").PresignGet(context.Background(), "k.png", 60*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Errorf("同输入应产出同 URL\na=%s\nb=%s", a, b)
	}
}

// TestPresignGetUnconfigured 未配 presign_domain 时必须报错,由调用方回落流式。
func TestPresignGetUnconfigured(t *testing.T) {
	_, err := presignDriver(t, "").PresignGet(context.Background(), "k.png", 60*time.Second)
	if err != ErrPresignUnconfigured {
		t.Errorf("err=%v,期望 ErrPresignUnconfigured", err)
	}
}

// TestPresignCredentialSlashEncoded scope 里的 / 必须是 %2F,否则签名串与
// 服务端重算的不一致。
func TestPresignCredentialSlashEncoded(t *testing.T) {
	raw, err := presignDriver(t, "https://s3.img.li").PresignGet(context.Background(), "k.png", 60*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(raw, "X-Amz-Credential=AKID%2F20260723%2Fus-east-1%2Fs3%2Faws4_request") {
		t.Errorf("Credential 未按 %%2F 编码: %s", raw)
	}
}

// TestPresignDomainNormalization 主机名小写、默认端口剥离后须与浏览器 authority
// 规范化一致,否则 SigV4 host 头对不上会静默 403。
func TestPresignDomainNormalization(t *testing.T) {
	canonical, err := presignDriver(t, "https://s3.img.li").PresignGet(context.Background(), "k.png", 60*time.Second)
	if err != nil {
		t.Fatal(err)
	}

	// 大写主机名、显式 :443、根路径 / 均应规范为 s3.img.li,且签名与纯 origin 一致。
	for _, domain := range []string{
		"https://S3.IMG.LI",
		"https://s3.img.li:443",
		"https://s3.img.li/",
	} {
		t.Run(domain, func(t *testing.T) {
			raw, err := presignDriver(t, domain).PresignGet(context.Background(), "k.png", 60*time.Second)
			if err != nil {
				t.Fatalf("PresignGet: %v", err)
			}
			u, err := url.Parse(raw)
			if err != nil {
				t.Fatalf("产出的不是合法 URL: %v", err)
			}
			if u.Host != "s3.img.li" {
				t.Errorf("Host=%q, want s3.img.li", u.Host)
			}
			if raw != canonical {
				t.Errorf("签名应与 https://s3.img.li 一致\ngot  %s\nwant %s", raw, canonical)
			}
		})
	}

	t.Run("http default port 80", func(t *testing.T) {
		raw, err := presignDriver(t, "http://h:80").PresignGet(context.Background(), "k.png", 60*time.Second)
		if err != nil {
			t.Fatalf("PresignGet: %v", err)
		}
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		if u.Scheme != "http" || u.Host != "h" {
			t.Errorf("got %s://%s, want http://h", u.Scheme, u.Host)
		}
	})

	t.Run("non-default port kept", func(t *testing.T) {
		raw, err := presignDriver(t, "https://s3.img.li:9000").PresignGet(context.Background(), "k.png", 60*time.Second)
		if err != nil {
			t.Fatalf("PresignGet: %v", err)
		}
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		if u.Host != "s3.img.li:9000" {
			t.Errorf("Host=%q, want s3.img.li:9000", u.Host)
		}
	})
}

// TestPresignDomainReject 非纯 origin / 非 ASCII 主机名必须在 New 时拒绝,
// 避免静默丢弃组件或签出必然 403 的 URL。
func TestPresignDomainReject(t *testing.T) {
	base := map[string]string{
		"endpoint": "192.0.2.10:9000", "region": "us-east-1", "bucket": "imgli",
		"access_key_id": "AKID", "secret_access_key": "SECRET",
		"path_style": "true",
	}
	cases := []string{
		"https://例子.com",                   // 非 ASCII 主机名
		"https://u:p@s3.img.li",              // 内联 userinfo
		"https://s3.img.li/some/path",        // 非根 path
		"https://s3.img.li?x=1",              // query
		"https://s3.img.li#f",                // fragment
		"https://u:p@s3.img.li/some/path?x=1#f",
	}
	for _, pd := range cases {
		t.Run(pd, func(t *testing.T) {
			cfg := map[string]string{}
			for k, v := range base {
				cfg[k] = v
			}
			cfg["presign_domain"] = pd
			_, err := New(cfg)
			if err == nil {
				t.Errorf("presign_domain %q 应报错", pd)
			}
		})
	}
}
