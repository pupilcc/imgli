package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yixian-huang/imgli/internal/config"
	"github.com/yixian-huang/imgli/internal/model"
)

func TestCheckBaseURL(t *testing.T) {
	lv, _ := CheckBaseURL("")
	if lv != Fail {
		t.Fatal("empty fail")
	}
	lv, msg := CheckBaseURL("https://img.li")
	if lv != OK || !strings.Contains(msg, "img.li") {
		t.Fatalf("%v %q", lv, msg)
	}
	lv, _ = CheckBaseURL("http://localhost:8686")
	if lv != Warn {
		t.Fatalf("localhost want warn got %v", lv)
	}
	lv, _ = CheckBaseURL("ftp://x")
	if lv != Fail {
		t.Fatal("ftp fail")
	}
}

func TestRunHappyLocal(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		Listen:  "127.0.0.1:0",
		BaseURL: "https://img.li",
		DataDir: dir,
		Database: config.Database{
			Driver: "sqlite",
			DSN:    filepath.Join(dir, "t.db"),
		},
		TrustProxy: false,
	}
	// seed via open+migrate+seed
	db, err := model.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := model.Migrate(db); err != nil {
		t.Fatal(err)
	}
	if err := model.Seed(db); err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	_ = sqlDB.Close()

	rep := Run(cfg)
	out := Format(rep)
	t.Log(out)
	if rep.HardFail {
		t.Fatalf("unexpected hard fail: %s", out)
	}
	// data dir should be ok
	found := false
	for _, c := range rep.Checks {
		if c.Name == "data_dir" && c.Level == OK {
			found = true
		}
	}
	if !found {
		t.Fatal("missing data_dir ok")
	}
	// cleanup probe files
	_ = os.Remove(filepath.Join(dir, ".imgli-doctor-write"))
}

func TestRunDataDirNotWritable(t *testing.T) {
	// skip if root
	if os.Getuid() == 0 {
		t.Skip("root")
	}
	// use a file as data_dir path parent that is not a dir
	f := filepath.Join(t.TempDir(), "notadir")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		Listen:   ":8686",
		BaseURL:  "http://localhost:8686",
		DataDir:  f,
		Database: config.Database{Driver: "sqlite", DSN: filepath.Join(t.TempDir(), "x.db")},
	}
	rep := Run(cfg)
	if !rep.HardFail {
		t.Fatal("want hard fail for non-dir data_dir")
	}
}
