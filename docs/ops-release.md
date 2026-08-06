# 发版与 CI/CD（v0.9.5+）

产品版本来自 **git tag**（`vMAJOR.MINOR.PATCH`）。  
GitHub Actions：`ci.yml`（PR/main）· `release.yml`（tag）· `smoke-prod.yml`（可选公网冒烟）。

## 推荐发版流程

```text
1. PR → CI 全绿（含 e2e-smoke）
2. merge → main
3. 本地: ./scripts/pre-tag-check.sh v0.9.6   # 校验 CHANGELOG + main 绿
4. git tag -a v0.9.6 -m "…" && git push origin v0.9.6
5. 等 release workflow:
     - goreleaser 完成 → 二进制可用（生产主要用这个）
     - docker-amd64 完成 → ghcr 单架构可先拉
     - docker-multi 完成 → 多架构 manifest 齐套
6. 生产: ./scripts/ops-deploy-baili.sh v0.9.6
   或: npc server exec command "VIP Cloud" -- bash -s < scripts/ops-deploy-baili.sh
7. 冒烟: ./scripts/ops-smoke-public.sh https://img.li
   或: Actions → smoke-prod → Run workflow
```

## 产物时序（为何拆开）

| 信号 | 含义 | 典型耗时 |
|------|------|----------|
| `goreleaser` 绿 | GitHub Release 资产可用，`install.sh` / 手装二进制 | ~3–5 min |
| `docker-amd64` 绿 | `ghcr.io/…:vX.Y.Z` 至少 amd64 可用 | 中等 |
| `docker-multi` 绿 | amd64+arm64 manifest 完整 | 最长（~10–15 min） |

**img.li 生产**当前为 VIP 上 systemd `baili` + `/opt/baili/bin/imgli` 二进制，**不必等 Docker multi 完成**。

## 脚本一览

| 脚本 | 用途 |
|------|------|
| `scripts/pre-tag-check.sh <tag>` | CHANGELOG 版本段、工作区干净、main 上最新 CI success |
| `scripts/ops-deploy-baili.sh [tag]` | 下载 Release 二进制 → 备份 → 安装 → restart → 本机/公网 health |
| `scripts/ops-smoke-public.sh [base_url]` | root / healthz / JS bundle / config 抽检（防白屏漏报） |

## Makefile 前端目标

| 目标 | 行为 |
|------|------|
| `make web` | `npm install` + `npm run build`（本地） |
| `make web-ci` | 仅 `npm run build`（CI 已 `npm ci` 时用，避免重复 install） |

## CI 策略摘要

- **concurrency**：同 ref 新 push 取消进行中的 CI。
- **e2e-smoke**（PR + main）：`guest` / `main` / `lang` / `pwa` 快路径。
- **e2e**（仅 main push）：全量 Playwright；PR 上可手跑 `npm run e2e`。
- **release**：goreleaser ∥ docker-amd64 ∥ docker-multi；失败互不阻塞「二进制是否已可用」。
