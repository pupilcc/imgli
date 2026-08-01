package webdav

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestChildNamesFromPropfindOpenListShape(t *testing.T) {
	// URL-encoded non-ASCII mount segment (e.g. UTF-8 "相册")
	const enc = "%E7%9B%B8%E5%86%8C" // 相册
	const name = "相册"
	xml := `<?xml version="1.0" encoding="UTF-8"?><D:multistatus xmlns:D="DAV:">` +
		`<D:response><D:href>/dav/</D:href><D:propstat><D:prop><D:resourcetype><D:collection/></D:resourcetype>` +
		`<D:displayname>root</D:displayname></D:prop></D:propstat></D:response>` +
		`<D:response><D:href>/dav/` + enc + `/</D:href><D:propstat><D:prop>` +
		`<D:resourcetype><D:collection/></D:resourcetype><D:displayname>` + name + `</D:displayname></D:prop></D:propstat></D:response>` +
		`</D:multistatus>`
	names := childNamesFromPropfind("https://dav.example/dav", xml)
	if len(names) != 1 || names[0] != name {
		t.Fatalf("names=%v want [%s]", names, name)
	}
	names2 := childNamesFromPropfind("https://dav.example/dav/"+name, xml)
	if len(names2) != 0 {
		t.Fatalf("under mount base names=%v want empty", names2)
	}
}

func TestRelativeChildName(t *testing.T) {
	cases := []struct {
		base, href, want string
	}{
		{"/dav", "/dav/", ""},
		{"/dav", "/dav", ""},
		{"/dav", "/dav/photos/", "photos"},
		{"/dav", "/dav/%E7%9B%B8%E5%86%8C/", "相册"},
		{"/dav", "https://dav.example/dav/foo/bar", "foo"},
		{"/dav/foo", "/dav/foo/bar", "bar"},
		{"/dav", "/other", ""},
	}
	for _, c := range cases {
		if got := relativeChildName(c.base, c.href); got != c.want {
			t.Errorf("relativeChildName(%q,%q)=%q want %q", c.base, c.href, got, c.want)
		}
	}
}

func TestSuggestWritableMounts(t *testing.T) {
	var mu sync.Mutex
	objects := map[string][]byte{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch {
		case r.Method == "PROPFIND":
			// root listing
			if p == "/" || p == "" {
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(http.StatusMultiStatus)
				_, _ = io.WriteString(w, `<?xml version="1.0"?><D:multistatus xmlns:D="DAV:">`+
					`<D:response><D:href>/</D:href><D:propstat><D:prop><D:resourcetype><D:collection/></D:resourcetype></D:prop></D:propstat></D:response>`+
					`<D:response><D:href>/disk/</D:href><D:propstat><D:prop><D:resourcetype><D:collection/></D:resourcetype></D:prop></D:propstat></D:response>`+
					`<D:response><D:href>/readonly/</D:href><D:propstat><D:prop><D:resourcetype><D:collection/></D:resourcetype></D:prop></D:propstat></D:response>`+
					`</D:multistatus>`)
				return
			}
			w.WriteHeader(http.StatusMultiStatus)
			_, _ = io.WriteString(w, `<?xml version="1.0"?><D:multistatus xmlns:D="DAV:"></D:multistatus>`)
		case strings.HasPrefix(p, "/disk"):
			key := strings.TrimPrefix(strings.TrimPrefix(p, "/disk"), "/")
			mu.Lock()
			defer mu.Unlock()
			switch r.Method {
			case http.MethodPut:
				b, _ := io.ReadAll(r.Body)
				objects[key] = b
				w.WriteHeader(http.StatusCreated)
			case http.MethodDelete:
				delete(objects, key)
				w.WriteHeader(http.StatusNoContent)
			case http.MethodHead:
				if _, ok := objects[key]; !ok {
					http.NotFound(w, r)
					return
				}
				w.WriteHeader(http.StatusOK)
			case http.MethodGet:
				b, ok := objects[key]
				if !ok {
					http.NotFound(w, r)
					return
				}
				_, _ = w.Write(b)
			default:
				w.WriteHeader(http.StatusMethodNotAllowed)
			}
		case strings.HasPrefix(p, "/readonly"):
			if r.Method == http.MethodPut {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusOK)
		default:
			if r.Method == http.MethodPut {
				http.NotFound(w, r)
				return
			}
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	d, err := New(map[string]string{"endpoint": srv.URL, "username": "u", "password": "p"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	hints, err := d.SuggestWritableMounts(ctx, 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(hints) != 1 || hints[0].Name != "disk" {
		t.Fatalf("hints=%+v", hints)
	}
	if !strings.HasSuffix(hints[0].Endpoint, "/disk") {
		t.Fatalf("endpoint=%s", hints[0].Endpoint)
	}
	s := FormatWritableHints(hints)
	if !strings.Contains(s, "disk") || !strings.Contains(s, "请将 endpoint") {
		t.Fatalf("format=%s", s)
	}
}

func TestDiscoverWritableMountsUnreachable(t *testing.T) {
	_, err := DiscoverWritableMounts(context.Background(), map[string]string{
		"endpoint": "http://127.0.0.1:1",
	}, 3)
	if err == nil {
		t.Fatal("want error")
	}
}
