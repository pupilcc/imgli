package upload

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/settings"
)

func sha256File(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func sha256Bytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func solidPNGFile(t *testing.T, dir string, w, h int, c color.Color) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	p := filepath.Join(dir, fmt.Sprintf("solid-%d-%d.png", w, h))
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()
	return p
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, b, 0o600); err != nil {
		t.Fatal(err)
	}
}

func readStored(t *testing.T, svc *Service, res *Result) []byte {
	t.Helper()
	d, err := svc.res.Driver(res.Policy)
	if err != nil {
		t.Fatal(err)
	}
	rc, err := d.Open(context.Background(), res.File.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	b, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestBurnNoopKeepsBytes(t *testing.T) {
	svc, u, _ := setup(t)
	dir := t.TempDir()
	src := pngFile(t, dir, 80, 60)
	wantHash := sha256File(t, src)
	tmp := filepath.Join(dir, "upload.png")
	copyFile(t, src, tmp)

	res, err := svc.Save(context.Background(), tmp, "noop.png", u, Opts{}, "")
	if err != nil {
		t.Fatal(err)
	}
	stored := readStored(t, svc, res)
	if sha256Bytes(stored) != wantHash {
		t.Error("全关处理时落盘 sha256 应与源字节一致(未重编码)")
	}
	if res.File.Hash != wantHash {
		t.Errorf("file.Hash=%s want %s", res.File.Hash, wantHash)
	}
}

func TestBurnTextChangesHash(t *testing.T) {
	svc, u, _ := setup(t)
	dir := t.TempDir()
	src := pngFile(t, dir, 120, 80)
	tmp1 := filepath.Join(dir, "t1.png")
	copyFile(t, src, tmp1)

	res1, err := svc.Save(context.Background(), tmp1, "before.png", u, Opts{}, "")
	if err != nil {
		t.Fatal(err)
	}
	h1 := res1.File.Hash

	proc := DefaultProcessing()
	proc.TextWatermark.Enabled = true
	proc.TextWatermark.Text = "测试"
	if err := settings.New(svc.db).Set(model.SettingProcessing, proc); err != nil {
		t.Fatal(err)
	}

	tmp2 := filepath.Join(dir, "t2.png")
	copyFile(t, src, tmp2)
	res2, err := svc.Save(context.Background(), tmp2, "after.png", u, Opts{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if res2.Instant {
		t.Error("文字水印生效后同字节不应秒传")
	}
	if res2.File.Hash == h1 {
		t.Error("烧录后 file.Hash 应与全关时不同")
	}
}

func TestBurnUserWatermark(t *testing.T) {
	svc, u, _ := setup(t)
	wmDir := t.TempDir()
	svc.WatermarkDir = wmDir

	// 100×100 红 PNG 水印图
	markSrc := solidPNGFile(t, t.TempDir(), 100, 100, color.RGBA{R: 255, A: 255})
	markBytes, err := os.ReadFile(markSrc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wmDir, fmt.Sprintf("%d.png", u.ID)), markBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	u.Preferences.Watermark = model.WatermarkPref{
		Enabled: true, Position: "br", Opacity: 1, Margin: 4,
	}
	if err := svc.db.Model(u).Select("preferences").Updates(u).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.db.First(u, u.ID).Error; err != nil {
		t.Fatal(err)
	}

	res, err := svc.Save(context.Background(), solidPNGFile(t, t.TempDir(), 400, 400, color.White), "wm.png", u, Opts{}, "")
	if err != nil {
		t.Fatal(err)
	}
	stored := readStored(t, svc, res)
	img, err := png.Decode(bytes.NewReader(stored))
	if err != nil {
		t.Fatal(err)
	}
	// 右下角 region 非白(mark 100×100 margin 4 → 覆盖 [296,396)×[296,396))
	r, g, b, _ := img.At(350, 350).RGBA()
	if r>>8 < 150 || g>>8 > 120 {
		t.Errorf("用户水印 br 区域应偏红, got rgb=(%d,%d,%d)", r>>8, g>>8, b>>8)
	}

	// 游客:开 guest,同全局默认(无站点印)→ 用户印跳过,右下仍白
	if err := settings.New(svc.db).Set(model.SettingGuestUpload, true); err != nil {
		t.Fatal(err)
	}
	gres, err := svc.Save(context.Background(), solidPNGFile(t, t.TempDir(), 400, 400, color.White), "guest.png", nil, Opts{}, "8.8.8.8")
	if err != nil {
		t.Fatal(err)
	}
	gimg, err := png.Decode(bytes.NewReader(readStored(t, svc, gres)))
	if err != nil {
		t.Fatal(err)
	}
	gr, gg, gb, _ := gimg.At(350, 350).RGBA()
	if gr>>8 < 240 || gg>>8 < 240 || gb>>8 < 240 {
		t.Errorf("游客应跳过用户水印,右下仍白 got rgb=(%d,%d,%d)", gr>>8, gg>>8, gb>>8)
	}
}

func TestBurnMaxEdge(t *testing.T) {
	svc, u, _ := setup(t)
	proc := DefaultProcessing()
	proc.MaxEdge = 400
	if err := settings.New(svc.db).Set(model.SettingProcessing, proc); err != nil {
		t.Fatal(err)
	}

	res, err := svc.Save(context.Background(), pngFile(t, t.TempDir(), 800, 200), "edge.png", u, Opts{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if res.File.Width != 400 {
		t.Errorf("Width=%d, want 400(重探生效)", res.File.Width)
	}
	if res.File.Height != 100 {
		t.Errorf("Height=%d, want 100", res.File.Height)
	}
}

func TestBurnSkipsGIF(t *testing.T) {
	svc, u, _ := setup(t)
	// 开全部处理项
	proc := DefaultProcessing()
	proc.MaxEdge = 400
	proc.TextWatermark.Enabled = true
	proc.TextWatermark.Text = "水印"
	if err := settings.New(svc.db).Set(model.SettingProcessing, proc); err != nil {
		t.Fatal(err)
	}
	svc.WatermarkDir = t.TempDir()
	// 写水印图(即便有,GIF 也不应烧录)
	_ = os.WriteFile(filepath.Join(svc.WatermarkDir, fmt.Sprintf("%d.png", u.ID)),
		mustPNGBytes(t, 10, 10, color.RGBA{R: 255, A: 255}), 0o644)
	u.Preferences.Watermark = model.WatermarkPref{Enabled: true, Opacity: 1, Margin: 0}
	if err := svc.db.Model(u).Select("preferences").Updates(u).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.db.First(u, u.ID).Error; err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	gifPath := gifFile(t, dir)
	srcHash := sha256File(t, gifPath)
	tmp1 := filepath.Join(dir, "g1.gif")
	copyFile(t, gifPath, tmp1)
	res1, err := svc.Save(context.Background(), tmp1, "a.gif", u, Opts{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if res1.File.Hash != srcHash {
		t.Errorf("GIF 不应被处理, hash=%s want %s", res1.File.Hash, srcHash)
	}

	tmp2 := filepath.Join(dir, "g2.gif")
	copyFile(t, gifPath, tmp2)
	res2, err := svc.Save(context.Background(), tmp2, "b.gif", u, Opts{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if !res2.Instant {
		t.Error("GIF 字节未动,第二次应秒传 Instant==true")
	}
}

func gifFile(t *testing.T, dir string) string {
	t.Helper()
	img := image.NewPaletted(image.Rect(0, 0, 1, 1), color.Palette{color.White, color.Black})
	p := filepath.Join(dir, "in.gif")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := gif.Encode(f, img, nil); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()
	return p
}

func mustPNGBytes(t *testing.T, w, h int, c color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
