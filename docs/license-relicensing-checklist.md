# License change checklist (MIT → AGPL-3.0-only + commercial dual-license)

Use this when reviewing or repeating a relicensing. Owner: copyright holder.

## Decision (done for imgli)

| Item | Choice |
|------|--------|
| Community license | **AGPL-3.0-only** (SPDX) |
| Goal | 防白嫖：修改后作为网络服务提供须开源对应代码 |
| Commercial path | Dual license — paid proprietary grant ([COMMERCIAL.md](../COMMERCIAL.md)) |
| Historical tags | `v0.1.0` / `v0.1.1` stay **MIT** |

## Pre-flight

- [x] Sole / clear copyright for tree (no external PR backlog)
- [x] No meaningful forks/stars yet (best window to relicense)
- [x] Third-party deps compatible with AGPL distribution (typical MIT/BSD/Apache OK; re-check on major deps)
- [x] Fonts/assets licenses recorded ([NOTICE](../NOTICE), OFL)
- [ ] Optional: lawyer review of commercial agreement template (before first paid customer)

## Repository files

- [x] `LICENSE` — full AGPL-3.0 + project preamble + historical MIT note
- [x] `COMMERCIAL.md` — when to buy, contact, dual-license model
- [x] `NOTICE` — copyright + third-party
- [x] `README.md` / `README.zh-CN.md` — badge + License section
- [x] `CONTRIBUTING.md` — AGPL + relicense grant + DCO `-s`
- [x] `CHANGELOG.md` — Unreleased note
- [x] `SECURITY.md` — supported versions note
- [ ] GitHub repo **About → License** refreshes after push (automatic from `LICENSE`)
- [ ] imgli.com / docs footer later: AGPL + commercial link (docs on other host)

## Git / release

- [ ] Commit on `main` with clear message (`chore(license): relicense to AGPL-3.0-only`)
- [ ] Do **not** rewrite or delete MIT tags `v0.1.0` / `v0.1.1`
- [ ] Next feature release tag (e.g. `v0.2.0`) under AGPL only
- [ ] Release notes mention license change once
- [ ] GoReleaser / install script unchanged (no license logic required)

## Commercial license ops (before first sale)

- [ ] Written **commercial license agreement** (use case, granted rights, no AGPL for covered copies, term, fee, support)
- [ ] Define product SKU (e.g. single production domain / multi-tenant / OEM)
- [ ] Invoice / contract process; store signed PDF offline
- [ ] Optional: private builds or license key — **not required** for dual-license legal model; prefer contract over DRM at first
- [ ] Support policy: community Issues vs paid channel

## Contributor pipeline

- [ ] Require `git commit -s` (DCO) on PRs
- [ ] Reject “drive-by” large dumps without DCO / rights clarity
- [ ] Before dual-licensing code that includes many external authors: CLA or explicit relicense grant on file
- [ ] Never relicense third-party deps; only your code + inbound grants

## Communication (when you start publicity)

- [ ] One short FAQ: why AGPL, how commercial works, MIT tags still valid
- [ ] Do not claim “never commercial”; do claim “core remains source-available under AGPL”
- [ ] Pin issue or DISCUSSION for licensing questions if noise rises

## What AGPL does / does not do

| Does | Does not |
|------|----------|
| Pressure closed forks offered as a network service to share source | Stop pure internal self-host of unmodified builds from complying reasonably |
| Pair cleanly with **selling AGPL exceptions** | Prevent use of old MIT tags forever |
| Signal serious project | Replace a support business by itself |

## Rollback / mistakes

- Do not force-push license history away; add a clarifying commit if wording was wrong.
- If a dependency is AGPL-incompatible, replace the dependency; do not quietly violate.
