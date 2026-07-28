package moderation

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

func sha256hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func hmacSHA256(key, data []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(data)
	return m.Sum(nil)
}

// tencentTC3 返回 Authorization 头值。timestamp=Unix 秒字符串,date=UTC "2006-01-02"。
func tencentTC3(secretID, secretKey, service, host, timestamp, date string, payload []byte) string {
	canonicalHeaders := "content-type:application/json; charset=utf-8\nhost:" + host + "\n"
	signedHeaders := "content-type;host"
	canonicalRequest := "POST\n/\n\n" + canonicalHeaders + "\n" + signedHeaders + "\n" + sha256hex(payload)
	credentialScope := date + "/" + service + "/tc3_request"
	stringToSign := "TC3-HMAC-SHA256\n" + timestamp + "\n" + credentialScope + "\n" + sha256hex([]byte(canonicalRequest))
	secretDate := hmacSHA256([]byte("TC3"+secretKey), []byte(date))
	secretService := hmacSHA256(secretDate, []byte(service))
	secretSigning := hmacSHA256(secretService, []byte("tc3_request"))
	signature := hex.EncodeToString(hmacSHA256(secretSigning, []byte(stringToSign)))
	return "TC3-HMAC-SHA256 Credential=" + secretID + "/" + credentialScope +
		", SignedHeaders=" + signedHeaders + ", Signature=" + signature
}

// aliyunACS3 返回 Authorization 头值。timestamp=UTC "2006-01-02T15:04:05Z",nonce=随机十六进制。
func aliyunACS3(accessKeyID, accessKeySecret, host, action, version, nonce, timestamp string, payload []byte) string {
	hashedPayload := sha256hex(payload)
	canonicalHeaders := "content-type:application/json; charset=utf-8\nhost:" + host +
		"\nx-acs-action:" + action + "\nx-acs-content-sha256:" + hashedPayload +
		"\nx-acs-date:" + timestamp + "\nx-acs-signature-nonce:" + nonce + "\nx-acs-version:" + version + "\n"
	signedHeaders := "content-type;host;x-acs-action;x-acs-content-sha256;x-acs-date;x-acs-signature-nonce;x-acs-version"
	canonicalRequest := "POST\n/\n\n" + canonicalHeaders + "\n" + signedHeaders + "\n" + hashedPayload
	stringToSign := "ACS3-HMAC-SHA256\n" + sha256hex([]byte(canonicalRequest))
	signature := hex.EncodeToString(hmacSHA256([]byte(accessKeySecret), []byte(stringToSign)))
	return "ACS3-HMAC-SHA256 Credential=" + accessKeyID + ",SignedHeaders=" + signedHeaders + ",Signature=" + signature
}
