//go:build vips

package imaging

/*
#cgo pkg-config: vips
#include <vips/vips.h>
#include <stdlib.h>

// webp_from_png_buffer:pngload_buffer → webpsave_buffer(Q)，保留 alpha，不 flatten。
// 成功 0；失败 -1。*out 由 g_malloc，调用方 g_free。
static int webp_from_png_buffer(void *in, size_t in_len, int q, void **out, size_t *out_len) {
	VipsImage *im = NULL;
	int ret = -1;
	if (q < 1) {
		q = 1;
	}
	if (q > 100) {
		q = 100;
	}
	if (vips_pngload_buffer(in, in_len, &im, NULL)) {
		return -1;
	}
	if (vips_webpsave_buffer(im, out, out_len, "Q", q, NULL)) {
		goto done;
	}
	ret = 0;
done:
	if (im) {
		g_object_unref(im);
	}
	return ret;
}
*/
import "C"

import (
	"bytes"
	"image"
	"image/png"
	"unsafe"
)

// WebPEncodeAvailable vips 构建具备原图 WebP 编码能力。
func WebPEncodeAvailable() bool { return true }

// EncodeWebP 将任意 image.Image 经无损 PNG 中转后用 libvips 有损编码为 WebP。
// quality 0/非法 → 80。保留 alpha（不 flatten）。
func EncodeWebP(img image.Image, quality int) ([]byte, error) {
	if img == nil {
		return nil, ErrUnsupported
	}
	if err := ensureVips(); err != nil {
		return nil, err
	}
	q := clampWebPQuality(quality)
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		return nil, err
	}
	data := pngBuf.Bytes()
	if len(data) == 0 {
		return nil, ErrUnsupported
	}

	var out unsafe.Pointer
	var outLen C.size_t
	in := C.CBytes(data)
	defer C.free(in)

	if C.webp_from_png_buffer(in, C.size_t(len(data)), C.int(q), &out, &outLen) != 0 {
		C.vips_error_clear()
		return nil, ErrUnsupported
	}
	if out == nil || outLen == 0 {
		return nil, ErrUnsupported
	}
	buf := C.GoBytes(out, C.int(outLen))
	C.g_free(C.gpointer(out))
	return buf, nil
}
