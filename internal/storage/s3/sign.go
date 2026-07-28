package s3

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

func sha256hex(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func hmacSHA256(key, data []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(data)
	return m.Sum(nil)
}

// SignV4Raw 计算 SigV4 裸签名值与 SignedHeaders 串(headers 名小写+字典序)。
// header 签名与 query-string 预签名共用本函数:前者由 SignV4 包装成 Authorization
// 头,后者直接把签名拼进 X-Amz-Signature。payloadHash 为 UNSIGNED-PAYLOAD 或十六进制 sha256。
func SignV4Raw(method, canonicalURI, canonicalQuery, secretKey, region, service, amzDate, date, payloadHash string, headers map[string]string) (signature, signedHeaders string) {
	names := make([]string, 0, len(headers))
	lower := make(map[string]string, len(headers))
	for k, v := range headers {
		lk := strings.ToLower(k)
		names = append(names, lk)
		lower[lk] = v
	}
	sort.Strings(names)
	var ch strings.Builder
	for _, n := range names {
		ch.WriteString(n)
		ch.WriteString(":")
		// SigV4:头值首尾去空白 + 内部连续空白折叠为单空格(codex 评审 F5)。
		ch.WriteString(strings.Join(strings.Fields(lower[n]), " "))
		ch.WriteString("\n")
	}
	signedHeaders = strings.Join(names, ";")
	canonicalRequest := method + "\n" + canonicalURI + "\n" + canonicalQuery + "\n" +
		ch.String() + "\n" + signedHeaders + "\n" + payloadHash
	credentialScope := date + "/" + region + "/" + service + "/aws4_request"
	stringToSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + credentialScope + "\n" + sha256hex([]byte(canonicalRequest))
	kDate := hmacSHA256([]byte("AWS4"+secretKey), []byte(date))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))
	return hex.EncodeToString(hmacSHA256(kSigning, []byte(stringToSign))), signedHeaders
}

// SignV4 生成 AWS SigV4 Authorization 头。
func SignV4(method, canonicalURI, canonicalQuery, accessKey, secretKey, region, service, amzDate, date, payloadHash string, headers map[string]string) string {
	signature, signedHeaders := SignV4Raw(method, canonicalURI, canonicalQuery, secretKey,
		region, service, amzDate, date, payloadHash, headers)
	credentialScope := date + "/" + region + "/" + service + "/aws4_request"
	return "AWS4-HMAC-SHA256 Credential=" + accessKey + "/" + credentialScope +
		",SignedHeaders=" + signedHeaders + ",Signature=" + signature
}
