package handler

import (
	"net/http"
	"testing"

	"github.com/yixian-huang/imgli/internal/service/storagesvc"
)

func TestThumbWidthWhitelist(t *testing.T) {
	fx := newServeFixture(t)
	recBad := fx.get("/t/"+fx.name+"?w=123", map[string]string{"Accept": "application/json"})
	if recBad.Code != http.StatusBadRequest {
		t.Fatalf("bad w: %d body=%s", recBad.Code, recBad.Body.String())
	}
	recBad2 := fx.get("/t/"+fx.name+"?w=abc", map[string]string{"Accept": "application/json"})
	if recBad2.Code != http.StatusBadRequest {
		t.Fatalf("non-int w: %d", recBad2.Code)
	}
	rec0 := fx.get("/t/"+fx.name, nil)
	if rec0.Code != http.StatusOK {
		t.Fatalf("default thumb: %d", rec0.Code)
	}
	key := storagesvc.WidthThumbKey("public", "abc", 400)
	if key != "public/.thumbs/w400/g"+storagesvc.ThumbGen+"/abc.jpg" {
		t.Fatalf("WidthThumbKey=%q", key)
	}
}
