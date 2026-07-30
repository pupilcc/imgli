# Documentation single sources of truth (SSOT)

imgli documentation is **layered by audience**, not mirrored in full between
GitHub and the product knowledge base.

## Layers

| Layer | Location | Audience | What belongs here |
|-------|----------|----------|-------------------|
| **Engineering (Git)** | This repo `docs/`, `README*`, `CHANGELOG` | Contributors, self-hosters reading the source | Matrices, scripts, design drafts, ops checklists tied to code/tags |
| **Product docs (KB)** | omni paths `docs-imgli/*` → docs.imgli.com | End users / operators of the product UI | How-to, FAQ, install narrative, feature overview |
| **Internal (KB)** | omni `imgli/*`, `hub/imgli` | Maintainers only | Roadmap, production ops, commercial strategy, session decisions |

## Rules

1. **One duty → one edit place.** Summaries + links are OK; dual full copies are not.
2. **Behavior changed in a PR** → update **repo** `docs/` (and README/CHANGELOG as needed) in the same PR.
3. **After release / user-facing change** → refresh **`docs-imgli/*`** (and hub status if production version moved). Prefer linking to GitHub for long matrices.
4. **Conflict:** GitHub tag + `CHANGELOG` + latest Release win for product version and APIs.
5. **Never** put full internal roadmap or VIP secrets into `docs-imgli/`.
6. **WebDAV / S3 / FTP matrices** live in **repo** (`docs/webdav-compatibility.md`, `docs/s3-compatibility.md`, `docs/storage-ftp.md`); product site pages stay short and link out.

## Map (storage-related example)

| Topic | Repo SSOT | Product docs (`docs-imgli`) |
|-------|-----------|-------------------------------|
| S3 vendors | `docs/s3-compatibility.md` | `docs-imgli/s3` (summary) |
| WebDAV vendors | `docs/webdav-compatibility.md` | `docs-imgli/storage-cdn` + link (or `docs-imgli/webdav`) |
| FTP dual track | `docs/storage-ftp.md` | `docs-imgli/ftp` |
| Caps design | `docs/design/storage-caps-draft.md` | user-facing Caps described on storage pages only |
| Production VIP deploy | *(not in public repo detail)* | internal `imgli/ops-*` only |

## Agent / human checklist

- [ ] New feature → code + repo docs in one PR  
- [ ] Ship tag → hub `last_release` / production note + `docs-imgli` version pins  
- [ ] No second full matrix in KB  
