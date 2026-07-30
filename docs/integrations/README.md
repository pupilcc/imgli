# Integrations

Upload to imgli / [img.li](https://img.li) using the same HTTP contract:

| Item | Value |
|------|--------|
| URL | `{BASE_URL}/api/v1/upload` |
| Method | `POST` |
| Body | `multipart/form-data`, file field **`file`** |
| Auth | Header `Authorization: Bearer <API_TOKEN>` |
| URL path in JSON | **`data.links.url`** |

Create a token: **Settings → API Token** (scope `upload` or `full`). Plaintext is shown once.

| Guide | Client |
|-------|--------|
| [picgo.md](../picgo.md) | PicGo / PicGo-Core / Typora / VS Code |
| [sharex.md](sharex.md) | [ShareX](https://getsharex.com/) (Windows) |
| [upic.md](upic.md) | [uPic](https://github.com/gee1k/uPic) / PicList-style custom uploaders |
| README **CLI** | `imgli upload` (single file) · `imgli import-dir` (bulk directory) |

### CLI bulk import (`import-dir`)

```bash
export IMGLI_BASE_URL=https://your-host
export IMGLI_TOKEN='your-api-token'
imgli import-dir ./photos                 # recursive by default
imgli import-dir -dry-run ./photos        # list only
imgli import-dir -visibility private ./in
```

Flags: `-recursive` (default true), `-continue` (default true), `-base-url`, `-token`.
Reuses `POST /api/v1/upload` (instant-upload / content-hash on the server).

### Related HTTP surfaces (v0.3+)

| Surface | Notes |
|---------|--------|
| `GET /t/{key}.jpg?w=200\|400\|800` | Whitelisted thumbnail widths |
| `POST /api/v1/s/{key}/unlock` | Password-protected share unlock |
| `GET /api/v1/a/{id}` · `/a/{id}/images` | Public album visitor API |
| Outbound webhooks | Admin `GET/PUT /api/v1/admin/webhooks` |
| OIDC SSO | Admin `GET/PUT /api/v1/admin/oidc`; user start `/api/v1/auth/oidc/start` |

Product docs: [docs.imgli.com](https://docs.imgli.com) (when published from `docs-imgli/`).

Sample ShareX custom uploader: [imgli.sxcu.example](imgli.sxcu.example).

Replace `https://img.li` with your self-hosted `IMGLI_BASE_URL` everywhere.
