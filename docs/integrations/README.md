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
| README **CLI upload** | `imgli upload` |

Sample ShareX custom uploader: [imgli.sxcu.example](imgli.sxcu.example).

Replace `https://img.li` with your self-hosted `IMGLI_BASE_URL` everywhere.
