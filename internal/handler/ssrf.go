package handler

import (
	"fmt"
	"net"
	"net/url"
)

// ValidateFetchURL 校验远程抓取 URL：仅 http/https，且解析出的所有 IP 均非
// 私网/环回/链路本地/未指定地址（防 SSRF 打穿内网，见 spec §6 红线）。
//
// 这只是"快速失败"的前置校验：DNS 解析结果可能在校验后与实际拨号时不同
// （DNS rebinding），也可能因跟随重定向而指向另一个主机。真正的防线是
// upload.go 中抓取用 *http.Client 的拨号期校验（Transport.DialContext 的
// net.Dialer.Control 回调，对每一次实际 TCP 连接的已解析 IP 做同一 isPublicIP
// 判定）——那里才是权威防线，此函数仅用于提前给出清晰错误信息。
func ValidateFetchURL(raw string, allow []*net.IPNet) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("非法 URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("仅支持 http/https")
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("缺少主机名")
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("解析主机失败: %w", err)
	}
	for _, ip := range ips {
		if !isPublicIP(ip) && !ipAllowed(ip, allow) {
			return fmt.Errorf("拒绝内网/保留地址: %s", ip)
		}
	}
	return nil
}

// ipAllowed 判定 ip 是否命中运维配置的额外放行清单(host/CIDR)。
func ipAllowed(ip net.IP, allow []*net.IPNet) bool {
	for _, n := range allow {
		if n != nil && n.Contains(ip) {
			return true
		}
	}
	return false
}

// isPublicIP 判定 ip 是否为公网地址（非环回/私网/链路本地/未指定/组播/CGNAT）。
// 供 ValidateFetchURL 与 upload.go 的拨号期校验共用，避免逻辑重复漂移。
func isPublicIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
		return false
	}
	// 100.64.0.0/10 CGNAT
	if ip4 := ip.To4(); ip4 != nil && ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
		return false
	}
	return true
}
