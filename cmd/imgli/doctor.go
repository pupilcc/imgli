package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/yixian-huang/imgli/internal/config"
	"github.com/yixian-huang/imgli/internal/doctor"
)

func runDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	cfgPath := fs.String("config", "", "配置文件路径（可选；也可用环境变量）")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "用法: imgli doctor [-config path]")
		fmt.Fprintln(os.Stderr, "  检查 data 目录、数据库、base_url、trust_proxy、local 存储策略等常见误配。")
		fmt.Fprintln(os.Stderr, "  存在 FAIL 时 exit 1；仅 WARN 时 exit 0。")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return fmt.Errorf("doctor 不接受位置参数")
	}
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	rep := doctor.Run(cfg)
	fmt.Print(doctor.Format(rep))
	if rep.HardFail {
		os.Exit(1)
	}
	return nil
}
