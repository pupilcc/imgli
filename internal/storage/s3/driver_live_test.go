package s3

// 驱动面真机判据(B-③ 厂商矩阵)。门禁与 env 同 presign_live_test.go(IMGLI_TEST_S3_*)。
// 覆盖 spec §3.1 矩阵的驱动面行:Put / Open 顺序读 / Range-Seek / Exists / Delete 幂等。
// 桶自建自清(建桶 200/409 视为就绪,凭据无建桶权限时请预建专用测试桶)。

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"
)

// patternBody 生成内容可寻址的确定性字节串:第 i 字节 = i%251。
// Range-Seek 读到的任意片段都能按 offset 独立验证,不依赖整读。
func patternBody(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i % 251)
	}
	return b
}

// ensureBucket 确保测试桶存在,返回「本次是否新建」。
// 谁建谁删:预建桶(用户手工开好的专用测试桶)不归测试清理——否则跑一轮判据就把
// 用户的桶删了,重跑还要重开。
//
// 先 HEAD 探测再决定是否 PUT,两个原因(codex 评审):
//  1. AWS us-east-1 对「已拥有的桶」重复 PUT 也返 200(历史怪癖,非 409),
//     直接 PUT 无法区分「新建」与「早已存在」,会把预建桶误标为本次新建而被清理删掉;
//  2. 对象-only 凭据(无桶级权限)PUT 桶会 403,不该在对象能力验证之前就 Fatal。
func ensureBucket(t *testing.T, e *liveEnv) (created bool) {
	t.Helper()
	switch code := e.signedDo(t, "HEAD", "/"+e.bucket+"/", nil); code {
	case 200:
		return false // 已存在:预建或上轮遗留,不归本测试清理
	case 404:
		switch code := e.signedDo(t, "PUT", "/"+e.bucket+"/", nil); code {
		case 200:
			return true // HEAD 已排除「早已存在」,此 200 确为本次新建
		case 409:
			return false // HEAD 与 PUT 之间被并发创建
		default:
			t.Fatalf("建桶失败 status=%d(凭据无建桶权限?请预建专用测试桶后重跑)", code)
			return false
		}
	default:
		// 403 等:多为对象-only 凭据无桶级读权限。不尝试建桶、不注册清理,直接继续
		// ——桶应已预建;若真不存在,紧随其后的第一次对象操作会响亮失败。
		t.Logf("HEAD 桶返回 %d,视为桶已预建(对象-only 凭据),跳过建桶", code)
		return false
	}
}

// TestEnsureBucketLive 只确保桶存在,不清理(e2e 脚本在起 app 前调用,
// 抵消判据测试「自建自删」把桶带走的副作用)。
func TestEnsureBucketLive(t *testing.T) {
	e := liveOrSkip(t)
	if ensureBucket(t, e) {
		t.Logf("桶 %s 已创建", e.bucket)
	} else {
		t.Logf("桶 %s 已存在", e.bucket)
	}
}

func TestDriverSurfaceLive(t *testing.T) {
	e := liveOrSkip(t)
	ctx := context.Background()
	key := fmt.Sprintf("surface-probe-%d.bin", time.Now().UnixNano())
	const size = 1 << 20 // 1 MiB:足以让 Range 语义有意义
	body := patternBody(size)

	created := ensureBucket(t, e)
	t.Cleanup(func() {
		e.d.Delete(context.Background(), key)
		if created {
			e.signedDo(t, "DELETE", "/"+e.bucket+"/", nil)
		}
	})

	// 1) Put
	if err := e.d.Put(ctx, key, bytes.NewReader(body)); err != nil {
		t.Fatalf("Put 失败: %v", err)
	}
	t.Log("1 Put ✓")

	// 2) Exists:存在与不存在两向
	if ok, err := e.d.Exists(ctx, key); err != nil || !ok {
		t.Fatalf("Exists(已写对象) = %v,%v,期望 true", ok, err)
	}
	if ok, err := e.d.Exists(ctx, key+".missing"); err != nil || ok {
		t.Fatalf("Exists(不存在键) = %v,%v,期望 false 且无错", ok, err)
	}
	t.Log("2 Exists 两向 ✓")

	// 3) Open 顺序整读:字节一致
	rc, err := e.d.Open(ctx, key)
	if err != nil {
		t.Fatalf("Open 失败: %v", err)
	}
	got, err := io.ReadAll(rc)
	rc.Close()
	if err != nil || !bytes.Equal(got, body) {
		t.Fatalf("顺序整读不一致: len=%d err=%v", len(got), err)
	}
	t.Log("3 Open 顺序整读 ✓")

	// 4) Range-Seek:offset>0 走 rangeReadSeekCloser 的 Range 路径
	//    (driver.go: offset>0 收到 200 必须报错——防服务端忽略 Range 静默整读)
	rsc, err := e.d.Open(ctx, key)
	if err != nil {
		t.Fatalf("Open(为 Seek) 失败: %v", err)
	}
	defer rsc.Close()
	const off = 700 * 1024
	if n, err := rsc.Seek(off, io.SeekStart); err != nil || n != off {
		t.Fatalf("Seek(%d, Start) = %d,%v", off, n, err)
	}
	rest, err := io.ReadAll(rsc)
	if err != nil {
		t.Fatalf("Seek 后读失败(服务端可能不支持 Range): %v", err)
	}
	if !bytes.Equal(rest, body[off:]) {
		t.Fatalf("Range 读内容错位: len=%d,首字节 %d 期望 %d", len(rest), rest[0], body[off])
	}
	// Seek(0, End) 应返回对象大小(HEAD 语义)
	if n, err := rsc.Seek(0, io.SeekEnd); err != nil || n != size {
		t.Fatalf("Seek(0, End) = %d,%v,期望 %d", n, err, size)
	}
	t.Log("4 Range-Seek(offset 读校验 + SeekEnd=size)✓")

	// 5) Delete + 幂等:删除后 Exists false;再删不存在键不报错
	if err := e.d.Delete(ctx, key); err != nil {
		t.Fatalf("Delete 失败: %v", err)
	}
	// Exists 的错误不可吞:网络/权限错返回 (false, err) 会被误判成「已删除」假阳性
	if ok, err := e.d.Exists(ctx, key); err != nil {
		t.Fatalf("Delete 后 Exists 探测出错: %v", err)
	} else if ok {
		t.Fatal("Delete 后对象仍存在")
	}
	if err := e.d.Delete(ctx, key); err != nil {
		t.Fatalf("Delete 幂等性:删不存在键报错 %v,期望 nil", err)
	}
	t.Log("5 Delete + 幂等 ✓")
}

// TestKeyAbsentLive 断言指定键不存在(e2e 脚本删除段复用:
// app 物理删除任务跑完后,以 IMGLI_TEST_S3_EXPECT_ABSENT_KEY 传入 files.path 验桶已清)。
func TestKeyAbsentLive(t *testing.T) {
	e := liveOrSkip(t)
	key := os.Getenv("IMGLI_TEST_S3_EXPECT_ABSENT_KEY")
	if key == "" {
		t.Skip("未设 IMGLI_TEST_S3_EXPECT_ABSENT_KEY")
	}
	// 注意:该键是 DB files.path 原值(已含 public/ 等 surface 前缀);
	// 驱动会再拼 policy prefix,故 e2e 场景下 env 的 PREFIX 必须与被测策略一致。
	if ok, err := e.d.Exists(context.Background(), key); err != nil || ok {
		t.Fatalf("对象应已被物理删除: exists=%v err=%v key=%s", ok, err, key)
	}
}
