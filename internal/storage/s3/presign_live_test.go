package s3

// 预签名 GET 的真机验证(B-⑤ spike / B-③ 厂商矩阵复用)。
//
// 门禁:未设 IMGLI_TEST_S3_LIVE=1 即 skip,与 IMGLI_TEST_PG_DSN 同模式,
// 不进默认 CI 路径。
//
// 需要的环境变量:
//
//	IMGLI_TEST_S3_LIVE=1
//	IMGLI_TEST_S3_ENDPOINT   如 http://127.0.0.1:9000(可带 http:// 前缀)
//	IMGLI_TEST_S3_REGION     如 us-east-1
//	IMGLI_TEST_S3_AK / _SK   凭据(需有建桶权限)
//	IMGLI_TEST_S3_BUCKET     可选,默认 imgli-presign-spike;**必须是本测试专用的新桶**
//	IMGLI_TEST_S3_PATHSTYLE  可选 "true"/"false",默认 true
//	IMGLI_TEST_S3_PRESIGN_DOMAIN 可选,预签名目标域;未设时回落为 endpoint(补 scheme)
//	IMGLI_TEST_S3_PREFIX     可选,对象键前缀(B-③ 矩阵验证 prefix 拼接用)
//
// 本测试自建桶、自建对象、结束时清理,不触碰任何既有桶。
// 判据走 Driver.PresignGet 真方法,覆盖 bucket/prefix/host/URL 组装逻辑。

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

type liveEnv struct {
	d      *Driver
	bucket string
	region string
	ak, sk string
}

func liveOrSkip(t *testing.T) *liveEnv {
	t.Helper()
	if os.Getenv("IMGLI_TEST_S3_LIVE") != "1" {
		t.Skip("未设 IMGLI_TEST_S3_LIVE=1,跳过真机预签名验证")
	}
	bucket := os.Getenv("IMGLI_TEST_S3_BUCKET")
	if bucket == "" {
		bucket = "imgli-presign-spike"
	}
	pathStyle := os.Getenv("IMGLI_TEST_S3_PATHSTYLE")
	if pathStyle == "" {
		pathStyle = "true"
	}
	endpoint := os.Getenv("IMGLI_TEST_S3_ENDPOINT")
	// presign_domain 未设时回落为 endpoint 本身(直连端点即合法签名域)。
	// 裸 endpoint 补 https://——与驱动语义一致(driver.go:缺省 https,http:// 须显式),
	// 否则厂商裸域名会被降级为明文 HTTP,预签名 URL(含可重放签名)裸奔(codex 评审)。
	// 本地 MinIO/RustFS 等 http 端点在 env 里写显式 http:// 前缀即可。
	presignDomain := os.Getenv("IMGLI_TEST_S3_PRESIGN_DOMAIN")
	if presignDomain == "" {
		lower := strings.ToLower(endpoint)
		if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
			presignDomain = endpoint
		} else {
			presignDomain = "https://" + endpoint
		}
	}
	cfg := map[string]string{
		"endpoint":          endpoint,
		"region":            os.Getenv("IMGLI_TEST_S3_REGION"),
		"bucket":            bucket,
		"access_key_id":     os.Getenv("IMGLI_TEST_S3_AK"),
		"secret_access_key": os.Getenv("IMGLI_TEST_S3_SK"),
		"path_style":        pathStyle,
		"presign_domain":    presignDomain,
		"prefix":            os.Getenv("IMGLI_TEST_S3_PREFIX"),
	}
	d, err := New(cfg)
	if err != nil {
		t.Fatalf("构造驱动失败(检查 IMGLI_TEST_S3_* 是否齐全): %v", err)
	}
	return &liveEnv{d: d, bucket: bucket, region: cfg["region"], ak: cfg["access_key_id"], sk: cfg["secret_access_key"]}
}

// signedDo 发一个普通 SigV4 header 签名请求(建桶/删桶用,驱动没暴露这些方法)。
func (e *liveEnv) signedDo(t *testing.T, method, uri string, body []byte) int {
	t.Helper()
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	date := now.Format("20060102")
	payload := sha256hex(body)
	host := e.d.endpoint

	hdrs := map[string]string{
		"host":                 host,
		"x-amz-content-sha256": payload,
		"x-amz-date":           amzDate,
	}
	auth := SignV4(method, uri, "", e.ak, e.sk, e.region, "s3", amzDate, date, payload, hdrs)

	req, err := http.NewRequest(method, e.d.scheme+"://"+host+uri, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("建请求失败: %v", err)
	}
	for k, v := range hdrs {
		if k != "host" {
			req.Header.Set(k, v)
		}
	}
	req.Header.Set("Authorization", auth)
	req.ContentLength = int64(len(body))

	resp, err := e.d.httpClient().Do(req)
	if err != nil {
		t.Fatalf("%s %s 失败: %v", method, uri, err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return resp.StatusCode
}

func getStatus(t *testing.T, url string) int {
	t.Helper()
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Get(url)
	if err != nil {
		t.Fatalf("GET 失败: %v", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return resp.StatusCode
}

// TestPresignGetLive 是 B-⑤ 的核心判据,也是 B-③ 四厂商矩阵的复用入口。
//
// 五段判据缺一不可——尤其 A 段:若匿名 GET 也返 200,说明桶是公共读,
// 后面所有 200 都不能证明签名生效(假阳性)。生产桶 imgli 当前正是这种状态,
// 故本测试**必须跑在专用新桶上**。
//
// 签名走 Driver.PresignGet 真方法(不再手拼 canonical query),覆盖 bucket 拼法、
// prefix、host、URL 组装与编码。
func TestPresignGetLive(t *testing.T) {
	e := liveOrSkip(t)
	ctx := context.Background()
	key := fmt.Sprintf("presign-probe-%d.txt", time.Now().UnixNano())
	body := []byte("imgli presign spike\n")

	// 建桶(已存在则 409 也算就绪);谁建谁删,预建桶不归测试清理。
	created := ensureBucket(t, e)
	t.Cleanup(func() {
		e.d.Delete(context.Background(), key)
		if created {
			e.signedDo(t, "DELETE", "/"+e.bucket+"/", nil)
		}
	})

	if err := e.d.Put(ctx, key, bytes.NewReader(body)); err != nil {
		t.Fatalf("写探针对象失败: %v", err)
	}

	_, uri := e.d.hostAndURI(key)
	plainURL := e.d.scheme + "://" + e.d.endpoint + uri

	// A. 对照组:匿名裸 GET 必须被拒。这一段决定后面几段是否有意义。
	if code := getStatus(t, plainURL); code == 200 {
		t.Fatalf("匿名裸 GET 返 200——该桶是公共读,预签名判据全部失效。"+
			"请换一个未配匿名策略的新桶再测(当前 bucket=%s)", e.bucket)
	} else {
		t.Logf("A 匿名裸 GET = %d (期望 403/401)", code)
	}

	// B. 有效签名必须放行——这才是「RustFS/该厂商支持 query-string 预签名」的证据。
	// 走 Driver.PresignGet 真方法,覆盖 bucket/prefix/host/URL 组装。
	valid, err := e.d.PresignGet(ctx, key, 60*time.Second)
	if err != nil {
		t.Fatalf("PresignGet 失败: %v", err)
	}
	if code := getStatus(t, valid); code != 200 {
		t.Fatalf("B 有效预签名 GET = %d,期望 200——该实现不支持 SigV4 query-string 预签名", code)
	}
	t.Logf("B 有效预签名 GET = 200 ✓")

	// C. 篡改签名必须被拒——排除「服务端根本没验签,只是碰巧放行」。
	tampered := valid[:len(valid)-4] + "dead"
	if code := getStatus(t, tampered); code == 200 {
		t.Fatalf("C 篡改签名仍返 200——服务端未真正校验签名")
	} else {
		t.Logf("C 篡改签名 = %d (期望 403) ✓", code)
	}

	// D. 过期签名必须被拒——TTL 60s 的安全语义靠这一条兜底。
	// PresignGet 用 d.nowOr(),注入 now 为 90s 前 + ttl=1s 即构造已过期 URL。
	if testing.Short() {
		t.Log("D 过期验证:-short 模式跳过")
		return
	}
	origNow := e.d.now
	e.d.now = func() time.Time { return time.Now().Add(-90 * time.Second) }
	expired, err := e.d.PresignGet(ctx, key, 1*time.Second)
	e.d.now = origNow
	if err != nil {
		t.Fatalf("PresignGet(过期) 失败: %v", err)
	}
	if code := getStatus(t, expired); code == 200 {
		t.Fatalf("D 已过期签名仍返 200——TTL 未被服务端强制,60s 安全语义不成立")
	} else {
		t.Logf("D 过期签名 = %d (期望 403) ✓", code)
	}
}
