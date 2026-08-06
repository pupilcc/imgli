package imaging

import (
	"runtime"
	"testing"
)

func TestVipsConcurrencyDefaultCap(t *testing.T) {
	t.Setenv("VIPS_CONCURRENCY", "")
	// Clear explicit empty may not unset — use unset
	t.Setenv("VIPS_CONCURRENCY", "")
	_ = runtime.GOMAXPROCS(0)
	n := VipsConcurrency()
	if n < 1 || n > 2 {
		t.Fatalf("default VipsConcurrency() = %d, want 1..2", n)
	}
}

func TestVipsConcurrencyEnvOverride(t *testing.T) {
	t.Setenv("VIPS_CONCURRENCY", "1")
	if VipsConcurrency() != 1 {
		t.Fatalf("got %d want 1", VipsConcurrency())
	}
	t.Setenv("VIPS_CONCURRENCY", "0")
	if VipsConcurrency() != 0 {
		t.Fatalf("got %d want 0", VipsConcurrency())
	}
}
