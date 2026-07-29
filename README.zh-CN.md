<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/brand/svg/lockup-dark.svg">
    <img src="docs/brand/svg/lockup-light.svg" alt="imgli" width="360">
  </picture>
</p>

<p align="center"><b>自托管图床——一跃成链。</b></p>

<p align="center">
  <a href="LICENSE"><img alt="MIT" src="https://img.shields.io/badge/license-MIT-blue.svg"></a>
  <a href="https://github.com/yixian-huang/imgli/releases/latest"><img alt="Release" src="https://img.shields.io/github/v/release/yixian-huang/imgli"></a>
  <a href=".github/workflows/ci.yml"><img alt="CI" src="https://github.com/yixian-huang/imgli/actions/workflows/ci.yml/badge.svg"></a>
</p>

<p align="center"><a href="README.md">English</a> · 简体中文</p>

**imgli** 是 Go 编写的单二进制图床,内嵌 React 前端。截图上传,直接得链接。
公共实例:[img.li](https://img.li)。

## 特性

- **单二进制**:前端 `go:embed` 内嵌;默认 SQLite、支持 PostgreSQL,无 CGO 依赖。
- **多存储**:本地盘、**S3 兼容**(MinIO/RustFS 已真机验证,附厂商验证工具包)、WebDAV;
  策略级 CDN 域 `302` 卸带宽,私密图预签名直连。
- **内容安全**:可插拔机审链——NSFW 检测端点 + OCR 词表筛查
  (自托管旁路服务见 `deploy/ocr-paddle/`),审核队列,按用户组配策略。
- **账号与分享**:用户组配额/限速、游客上传、邀请码、SMTP 邮件
  (验证/重置/拒审通知)、相册、公开画廊、回收站、图片过期。
- **生态对接**:干净的上传 API + API Token;PicGo/Typora/VS Code
  开箱即用([指南](docs/picgo.md))。
- **细节**:中英双语界面、PWA、深色模式、文字水印(内嵌中文字体子集)、
  带审计日志的管理后台。

## 快速开始

### Docker（预构建镜像）

```bash
docker run --rm -p 8686:8686 -v imgli-data:/data \
  -e IMGLI_BASE_URL=http://localhost:8686 \
  ghcr.io/yixian-huang/imgli:latest
# → http://localhost:8686（第一个注册用户即管理员）
```

固定版本用 `ghcr.io/yixian-huang/imgli:v0.1.0`（见
[Releases](https://github.com/yixian-huang/imgli/releases)）。

### Docker Compose

```bash
git clone https://github.com/yixian-huang/imgli && cd imgli
docker compose up -d
# → http://localhost:8686 (第一个注册用户即管理员)
```

### 源码构建

```bash
make build          # 需要 Go ≥ 1.26、Node ≥ 24
./imgli version     # ldflags 注入的 git tag，如 v0.1.0
./imgli serve       # → http://localhost:8686
```

各平台二进制见 [GitHub Release](https://github.com/yixian-huang/imgli/releases)。

## 配置

这里只有运维层配置——产品层(站点名、注册模式、用户组、存储策略、SMTP、机审)
全部在管理后台设置。

优先级:默认值 → YAML(`imgli serve -config imgli.yaml`,
见 [`deploy/imgli.example.yaml`](deploy/imgli.example.yaml))→ 环境变量:

| 环境变量 | 默认 | 含义 |
|---|---|---|
| `IMGLI_LISTEN` | `:8686` | 监听地址 |
| `IMGLI_BASE_URL` | `http://localhost:8686` | 生成外链的基础地址 |
| `IMGLI_DATA_DIR` | `./data` | 本地存储与 SQLite 目录 |
| `IMGLI_DATABASE_DRIVER` | `sqlite` | `sqlite` \| `postgres` |
| `IMGLI_DATABASE_DSN` | `<data_dir>/imgli.db` | postgres 时的 DSN |
| `IMGLI_TRUST_PROXY` | `false` | 信任 `X-Forwarded-For`(仅在可信反代后开) |
| `IMGLI_FETCH_ALLOW` | *(空)* | URL 抓取上传额外放行的 host/CIDR |

## 上传 API

```bash
curl -X POST https://your-host/api/v1/upload \
  -H "Authorization: Bearer <API_TOKEN>" \
  -F file=@shot.png -F visibility=public
# → data.links.{url,markdown,html,bbcode,thumbnail_url}
```

Token 在 **设置 → API Token** 创建;PicGo/Typora/VS Code 配置见
[docs/picgo.md](docs/picgo.md)。

## 开发

```bash
make test        # go vet + go test(sqlite;设 IMGLI_TEST_PG_DSN 跑 postgres)
make test-web    # vitest
cd web && npm run e2e   # Playwright,会先构建二进制
```

参与贡献见 [CONTRIBUTING.md](CONTRIBUTING.md)（含版本号与发版流程）；
变更记录 [CHANGELOG.md](CHANGELOG.md)；安全问题走 [SECURITY.md](SECURITY.md)，勿发公开 issue。

## 许可

[MIT](LICENSE) © 2026 Yixian Huang。内嵌 Noto Sans SC 子集遵循
[SIL OFL](internal/imaging/fonts/OFL.txt)。
