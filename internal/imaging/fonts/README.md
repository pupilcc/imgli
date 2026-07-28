# 文字水印字体（Noto Sans SC 子集）

嵌入文件：`NotoSansSC-Regular.otf`（`go:embed`，见 `../font.go`）。

## 为何子集

完整 Noto Sans SC OTF 约 **8MB**，会直接打进单二进制。水印只需拉丁 + 标点 + 常用汉字，
按 **GB2312 一级汉字 + Latin/标点 + 仓库已出现 CJK** 子集后约 **0.9MB**（体积约 −89%）。

罕见汉字若未在 `charset.txt` 中，渲染会落 notdef（豆腐块）。需要扩覆盖时改 charset 后重跑子集脚本。

## 再生子集

```bash
# 准备完整源字体（勿提交 full 文件）
# https://github.com/notofonts/noto-cjk/releases → NotoSansSC-Regular.otf
cp /path/to/NotoSansSC-Regular.otf internal/imaging/fonts/NotoSansSC-Regular.full.otf

pip install fonttools
./scripts/subset-watermark-font.sh
# 或
./scripts/subset-watermark-font.sh /path/to/NotoSansSC-Regular.otf
```

`charset.txt` 为 UTF-8 无分隔字符表；改表后务必重跑脚本再提交 OTF。

## 许可

SIL Open Font License 1.1，见 `OFL.txt`。子集仍受 OFL 约束，不得单独售卖字体。
