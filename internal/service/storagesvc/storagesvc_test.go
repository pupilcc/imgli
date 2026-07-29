package storagesvc

import (
	"regexp"
	"testing"
	"time"

	"github.com/yixian-huang/imgli/internal/config"
	"github.com/yixian-huang/imgli/internal/model"
)

func TestRenderPath(t *testing.T) {
	r := New(&config.Config{DataDir: t.TempDir()}, nil)
	now := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC)
	p, err := r.RenderPath("{Y}/{m}/{d}/{uniqid}.{ext}", "png", now)
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`^2026/07/16/[0-9A-Za-z]{12}\.png$`)
	if !re.MatchString(p) {
		t.Errorf("path=%q 不匹配模板", p)
	}
	// 两次渲染 uniqid 不同
	p2, _ := r.RenderPath("{uniqid}.{ext}", "png", now)
	p3, _ := r.RenderPath("{uniqid}.{ext}", "png", now)
	if p2 == p3 {
		t.Error("uniqid 应随机")
	}
}

func TestDriverLocalAndCache(t *testing.T) {
	dir := t.TempDir()
	r := New(&config.Config{DataDir: dir}, nil)
	p := &model.StoragePolicy{ID: 1, Driver: "local", Config: map[string]string{"root": "uploads"}}
	d1, err := r.Driver(p)
	if err != nil {
		t.Fatal(err)
	}
	d2, _ := r.Driver(p)
	if d1 != d2 {
		t.Error("同策略应返回缓存的同一 driver 实例")
	}
	// s3 无 config → s3.New 缺 endpoint 报错(驱动已支持,但配置不全)
	if _, err := r.Driver(&model.StoragePolicy{ID: 2, Driver: "s3"}); err == nil {
		t.Error("s3 缺配置应报错")
	}
	// codex 终审:同 ID 但 config 变了(改 root)→ 指纹变化,应重建驱动而非沿用旧缓存
	pChanged := &model.StoragePolicy{ID: 1, Driver: "local", Config: map[string]string{"root": "other"}}
	d3, err := r.Driver(pChanged)
	if err != nil {
		t.Fatal(err)
	}
	if d3 == d1 {
		t.Error("config 改变后应重建驱动,不得沿用旧缓存")
	}
	// 再取同 changed config → 缓存命中同实例
	d4, _ := r.Driver(pChanged)
	if d3 != d4 {
		t.Error("同配置应缓存命中同实例")
	}
}

func TestLinkBase(t *testing.T) {
	r := New(&config.Config{BaseURL: "https://img.li/"}, nil)
	if b := r.LinkBase(&model.StoragePolicy{}); b != "https://img.li" {
		t.Errorf("无 CDN 仍返 BaseURL 去尾斜杠: %q", b)
	}
	// 裁决 5: 即使设了 CDNDomain,LinkBase 恒返 BaseURL(复制链走 app /i)
	if b := r.LinkBase(&model.StoragePolicy{CDNDomain: "https://cdn.img.li/"}); b != "https://img.li" {
		t.Errorf("设了 CDNDomain 仍应返 BaseURL: %q", b)
	}
}

func TestObjectURL(t *testing.T) {
	r := New(&config.Config{BaseURL: "https://img.li/"}, nil)
	if u := r.ObjectURL(&model.StoragePolicy{Config: map[string]string{}}, "a/b.png"); u != "" {
		t.Errorf("CDNDomain 空应返空串: %q", u)
	}
	p := &model.StoragePolicy{
		CDNDomain: "https://cdn.img.li/",
		Config:    map[string]string{"prefix": "p/"},
	}
	if u := r.ObjectURL(p, "a/b.png"); u != "https://cdn.img.li/p/a/b.png" {
		t.Errorf("ObjectURL with prefix: %q", u)
	}
	pEmpty := &model.StoragePolicy{
		CDNDomain: "https://cdn.img.li/",
		Config:    map[string]string{},
	}
	if u := r.ObjectURL(pEmpty, "a/b.png"); u != "https://cdn.img.li/a/b.png" {
		t.Errorf("ObjectURL empty prefix: %q", u)
	}
	// codex 终审:特殊字符对象键须编码,保留 / 层级(防 Location 路径语义被改)
	pSpecial := &model.StoragePolicy{CDNDomain: "https://cdn.img.li", Config: map[string]string{}}
	if u := r.ObjectURL(pSpecial, "d/a b?c#d.png"); u != "https://cdn.img.li/d/a%20b%3Fc%23d.png" {
		t.Errorf("ObjectURL 特殊字符编码: %q", u)
	}
	// S4: private surface 键永不拼 CDN
	if u := r.ObjectURL(pEmpty, "private/2026/07/x.png"); u != "" {
		t.Errorf("private/ 键应拒绝 CDN URL, got %q", u)
	}
	if u := r.ObjectURL(pEmpty, "private/.thumbs/g1/hh.jpg"); u != "" {
		t.Errorf("private 缩略图键应拒绝 CDN URL, got %q", u)
	}
	if u := r.ObjectURL(pEmpty, "public/2026/07/x.png"); u != "https://cdn.img.li/public/2026/07/x.png" {
		t.Errorf("public/ 键应允许 CDN, got %q", u)
	}
}

func TestCDNEligibleObjectKey(t *testing.T) {
	if CDNEligibleObjectKey("private/a.png") {
		t.Error("private/ 应不合格")
	}
	if CDNEligibleObjectKey("/private/a.png") {
		t.Error("前导斜杠 private 应不合格")
	}
	if !CDNEligibleObjectKey("public/a.png") {
		t.Error("public/ 应合格")
	}
	if !CDNEligibleObjectKey("2026/07/legacy.png") {
		t.Error("S1 前遗留无 surface 前缀路径仍允许 CDN（配合 visibility）")
	}
	if CDNEligibleObjectKey("") {
		t.Error("空键不合格")
	}
}
