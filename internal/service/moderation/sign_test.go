package moderation

import "testing"

func TestTencentTC3Golden(t *testing.T) {
	payload := []byte(`{"Service":"baselineCheck","ServiceParameters":"{\"imageUrl\":\"data:image/png;base64,AAAA\"}"}`)
	got := tencentTC3("AKIDzTESTSECRETID000000000", "TESTSECRETKEY00000000000000", "ims",
		"ims.tencentcloudapi.com", "1735689600", "2025-01-01", payload)
	want := "TC3-HMAC-SHA256 Credential=AKIDzTESTSECRETID000000000/2025-01-01/ims/tc3_request, " +
		"SignedHeaders=content-type;host, Signature=d0542d8ba1f8bfadcdf661fd9b0c3e1c65b85b57b5448e6f754404aea106ff53"
	if got != want {
		t.Errorf("TC3 签名不符\n got=%s\nwant=%s", got, want)
	}
}

func TestAliyunACS3Golden(t *testing.T) {
	payload := []byte(`{"Service":"baselineCheck","ServiceParameters":"{\"imageUrl\":\"data:image/png;base64,AAAA\"}"}`)
	got := aliyunACS3("TESTACCESSKEYID0000", "TESTACCESSKEYSECRET00000000", "green-cip.cn-shanghai.aliyuncs.com",
		"ImageModeration", "2022-03-02", "fixed-nonce-000000", "2025-01-01T00:00:00Z", payload)
	want := "ACS3-HMAC-SHA256 Credential=TESTACCESSKEYID0000," +
		"SignedHeaders=content-type;host;x-acs-action;x-acs-content-sha256;x-acs-date;x-acs-signature-nonce;x-acs-version," +
		"Signature=9a817a44a38bb7525eeb5be1f8036af3358c9c56a90c45ea4ac7990e5d9b1c9c"
	if got != want {
		t.Errorf("ACS3 签名不符\n got=%s\nwant=%s", got, want)
	}
}
