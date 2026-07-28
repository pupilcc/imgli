package handler

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSafeURLRejectsPrivateAndLoopback(t *testing.T) {
	bad := []string{
		"http://127.0.0.1/x", "http://localhost/x", "http://169.254.169.254/latest/meta-data",
		"http://10.0.0.5/x", "http://192.168.1.1/x", "http://[::1]/x", "ftp://example.com/x",
		"http://0.0.0.0/x",
	}
	for _, u := range bad {
		if err := ValidateFetchURL(u, nil); err == nil {
			t.Errorf("应拒绝 %q", u)
		}
	}
}

func TestSafeURLAllowsPublicHTTP(t *testing.T) {
	// 8.8.8.8 是公网地址；仅校验地址分类，不发起请求
	if err := ValidateFetchURL("https://8.8.8.8/image.png", nil); err != nil {
		t.Errorf("公网 https 应放行: %v", err)
	}
}

func mustCIDR(t *testing.T, s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestValidateFetchURLAllowlist(t *testing.T) {
	// 私网默认被拒
	if err := ValidateFetchURL("http://10.1.2.3/x.png", nil); err == nil {
		t.Fatal("私网应被拒(空允许清单)")
	}
	// 命中允许清单 → 放行(注意:该地址会真的尝试解析;用字面量 IP host 避免 DNS)
	allow := []*net.IPNet{mustCIDR(t, "10.0.0.0/8")}
	if err := ValidateFetchURL("http://10.1.2.3/x.png", allow); err != nil {
		t.Fatalf("命中允许清单应放行,却=%v", err)
	}
	// 公网始终放行
	if err := ValidateFetchURL("http://8.8.8.8/x.png", nil); err != nil {
		t.Fatalf("公网应放行,却=%v", err)
	}
	// 允许清单外的其它私网仍被拒
	if err := ValidateFetchURL("http://192.168.1.1/x.png", allow); err == nil {
		t.Fatal("清单外私网应仍被拒")
	}
}

// TestFetchClientDialTimeAllowBranch 证明拨号期 Control 回调的放行分支在真实
// TCP 连接上确实生效——httptest.NewServer 绑定 127.0.0.1（环回，默认拒绝）；
// 命中允许清单后应放行并成功拿到响应；不带允许清单则应保持默认严格并被拒。
// 这是"fails-open"最坏情形的反面验证：允许分支不能连误放行都做不到。
func TestFetchClientDialTimeAllowBranch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	_, loopback, err := net.ParseCIDR("127.0.0.0/8")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("命中允许清单应放行", func(t *testing.T) {
		client := NewFetchClient([]*net.IPNet{loopback})
		resp, err := client.Get(ts.URL)
		if err != nil {
			t.Fatalf("命中允许清单的环回地址应放行，却报错: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
	})

	t.Run("默认无允许清单应严格拒绝", func(t *testing.T) {
		client := NewFetchClient(nil)
		_, err := client.Get(ts.URL)
		if err == nil {
			t.Fatal("默认（空允许清单）应拒绝环回地址，却放行了")
		}
	})
}
