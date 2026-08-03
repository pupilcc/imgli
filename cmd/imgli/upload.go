package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yixian-huang/imgli/internal/cliupload"
)

func runUpload(args []string) error {
	fs := flag.NewFlagSet("upload", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	baseURL := fs.String("base-url", envOr("IMGLI_BASE_URL", ""), "图床 base URL（也可用 IMGLI_BASE_URL）")
	token := fs.String("token", envOr("IMGLI_TOKEN", ""), "API Token（也可用 IMGLI_TOKEN；游客上传可空）")
	format := fs.String("format", "url", "输出格式: url | markdown | json")
	visibility := fs.String("visibility", "", "可选 public|private")
	expiresIn := fs.Int("expires-in", 0, "可选有效期秒数，0=永久/省略")
	name := fs.String("name", "", "stdin 上传时的文件名（默认 stdin.png）")
	verbose := fs.Bool("verbose", false, "打印用户组有效期/次数限制（quota 或 guest config）")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "用法: imgli upload [flags] <file|->")
		fmt.Fprintln(os.Stderr, "  将本地文件或 stdin 上传到图床 API（POST /api/v1/upload）。")
		fmt.Fprintln(os.Stderr, "  环境变量: IMGLI_BASE_URL, IMGLI_TOKEN")
		fmt.Fprintln(os.Stderr, "  组策略见 docs/user-groups-lifecycle.md；-verbose 打印当前账号上限。")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return fmt.Errorf("需要恰好一个路径参数（文件或 - 表示 stdin）")
	}
	if err := cliupload.ValidateFormat(*format); err != nil {
		return err
	}
	vis := strings.TrimSpace(*visibility)
	if vis != "" && vis != "public" && vis != "private" {
		return fmt.Errorf("-visibility 须为 public 或 private")
	}

	pathArg := fs.Arg(0)
	var (
		r        io.ReadCloser
		filename string
	)
	if pathArg == "-" {
		filename = strings.TrimSpace(*name)
		if filename == "" {
			filename = "stdin.png"
		}
		r = io.NopCloser(os.Stdin)
	} else {
		f, err := os.Open(pathArg)
		if err != nil {
			return err
		}
		r = f
		filename = filepath.Base(pathArg)
		if n := strings.TrimSpace(*name); n != "" {
			filename = n
		}
	}
	defer r.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	if *verbose {
		if err := cliupload.PrintAccessLimits(ctx, *baseURL, *token); err != nil {
			fmt.Fprintf(os.Stderr, "verbose: 读取组限制失败: %v\n", err)
		}
		if *expiresIn > 0 {
			fmt.Fprintf(os.Stderr, "verbose: 本次 expires-in=%d\n", *expiresIn)
		} else {
			fmt.Fprintln(os.Stderr, "verbose: 本次未传 expires-in（永久或由服务端组策略补默认）")
		}
	}

	res, err := cliupload.Upload(ctx, cliupload.Opts{
		BaseURL:    *baseURL,
		Token:      *token,
		Filename:   filename,
		Visibility: vis,
		ExpiresIn:  *expiresIn,
	}, r)
	if err != nil {
		return err
	}
	out, err := cliupload.FormatOutput(*format, res)
	if err != nil {
		return err
	}
	fmt.Println(out)
	return nil
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
