package auth

import (
	"strings"
	"testing"
)

func TestDeriveChannel(t *testing.T) {
	if g := DeriveChannel(true, SignupMeta{UTMSource: "x"}); g != ChannelInvite {
		t.Fatalf("invite wins: %s", g)
	}
	if g := DeriveChannel(false, SignupMeta{UTMSource: "github"}); g != ChannelUTM {
		t.Fatalf("utm: %s", g)
	}
	if g := DeriveChannel(false, SignupMeta{RefererHost: "news.example"}); g != ChannelReferer {
		t.Fatalf("referer: %s", g)
	}
	if g := DeriveChannel(false, SignupMeta{}); g != ChannelDirect {
		t.Fatalf("direct: %s", g)
	}
}

func TestNormalizeSignupHost(t *testing.T) {
	cases := map[string]string{
		"":                         "",
		"Example.COM":              "example.com",
		"https://Foo.Bar/path?q=1": "foo.bar",
		"//cdn.x/y":                "cdn.x",
		"evil.com/phish":           "evil.com",
		"host:8080":                "host",
	}
	for in, want := range cases {
		if g := normalizeSignupHost(in); g != want {
			t.Errorf("normalize(%q)=%q want %q", in, g, want)
		}
	}
}

func TestSanitizeTruncates(t *testing.T) {
	long := strings.Repeat("a", 200)
	m := SignupMeta{UTMSource: long, RefererHost: long}.Sanitize()
	if len(m.UTMSource) > signupFieldMax {
		t.Fatalf("utm len %d", len(m.UTMSource))
	}
}
