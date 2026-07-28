package s3

import (
	"strings"
	"testing"
)

func TestSignV4AWSOfficialGET(t *testing.T) {
	empty := sha256hex(nil)
	headers := map[string]string{
		"host":                 "examplebucket.s3.amazonaws.com",
		"range":                "bytes=0-9",
		"x-amz-content-sha256": empty,
		"x-amz-date":           "20130524T000000Z",
	}
	got := SignV4("GET", "/test.txt", "", "AKIAIOSFODNN7EXAMPLE", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		"us-east-1", "s3", "20130524T000000Z", "20130524", empty, headers)
	if !strings.Contains(got, "Signature=f0e8bdb87c964420e857bd35b5d6ed310bd44f0170aba48dd91039c6036bdb41") {
		t.Errorf("SigV4 与 AWS 官方 GET golden 不符:\n%s", got)
	}
}

func TestSignV4PutUnsignedPayload(t *testing.T) {
	headers := map[string]string{
		"host":                 "my-bucket.s3.us-east-1.amazonaws.com",
		"x-amz-content-sha256": "UNSIGNED-PAYLOAD",
		"x-amz-date":           "20250101T000000Z",
	}
	got := SignV4("PUT", "/2026/07/19/abc.png", "", "AKIDEXAMPLE0000", "SECRETKEY0000000000000000",
		"us-east-1", "s3", "20250101T000000Z", "20250101", "UNSIGNED-PAYLOAD", headers)
	if !strings.Contains(got, "Signature=7422070053ace8b2e412c2b7a526ced21ca18db988f1061338d7d537cbc8fdb3") {
		t.Errorf("PUT UNSIGNED-PAYLOAD golden 不符:\n%s", got)
	}
}

// TestSignV4RawMatchesHeader 锁定拆分不改变签名结果:SignV4Raw 产出的裸签名
// 必须与 SignV4 拼进 Authorization 头的那一段逐字相同。
func TestSignV4RawMatchesHeader(t *testing.T) {
	headers := map[string]string{
		"host":                 "s3.us-east-1.amazonaws.com",
		"x-amz-content-sha256": "UNSIGNED-PAYLOAD",
		"x-amz-date":           "20260723T120000Z",
	}
	auth := SignV4("GET", "/bucket/key.png", "", "AKID", "SECRET",
		"us-east-1", "s3", "20260723T120000Z", "20260723", "UNSIGNED-PAYLOAD", headers)
	raw, signedHeaders := SignV4Raw("GET", "/bucket/key.png", "", "SECRET",
		"us-east-1", "s3", "20260723T120000Z", "20260723", "UNSIGNED-PAYLOAD", headers)

	if !strings.HasSuffix(auth, ",Signature="+raw) {
		t.Fatalf("裸签名与 Authorization 头不一致\nauth=%s\nraw=%s", auth, raw)
	}
	if !strings.Contains(auth, ",SignedHeaders="+signedHeaders+",") {
		t.Fatalf("SignedHeaders 不一致\nauth=%s\nsignedHeaders=%s", auth, signedHeaders)
	}
	if signedHeaders != "host;x-amz-content-sha256;x-amz-date" {
		t.Fatalf("SignedHeaders=%q 应为字典序三头", signedHeaders)
	}
}
