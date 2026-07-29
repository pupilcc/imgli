# Screenshots

Product UI captures for README demos. Regenerated from a local `imgli serve`
instance (demo seed user + sample upload). Not marketing mockups.

| File | Scene |
|------|--------|
| `01-upload.png` | Upload page (drag/drop, paste, options) |
| `02-library.png` | Image library |
| `03-share.png` | Public share landing `/s/{key}` |
| `04-api-tokens.png` | Settings → API tokens + client snippets |
| `05-admin.png` | Admin dashboard (review queue entry in nav) |
| `06-image-detail.png` | Image detail modal |

To refresh (dev):

```bash
# serve on :8765 with a fresh data dir, register, upload a sample, then:
node --input-type=module scripts/capture-readme-screenshots.mjs  # if added later
```
