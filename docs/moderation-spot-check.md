# Operator moderation spot-check path

This guide is for **self-hosted operators**. imgli does not lock you to a single cloud scorer: plug in NSFW endpoints, OCR keyword side-car, and/or human review.

## What ships in-tree

| Piece | Where |
|-------|--------|
| Plugin chain (webhook / Aliyun / Tencent / OpenAI / nsfwjs) | Admin → System settings → Moderation |
| OCR lexicon side path | Admin → OCR keywords + optional `deploy/ocr-paddle` |
| Review queue | Admin → Review / Moderation |
| Lexicon import notes | `docs/lexicon/README.md` |

## Suggested spot-check cadence

1. **Daily (busy public trial):** sample pending queue + a few recent **public** uploads.
2. **Weekly:** re-check rejected samples for false positives; refresh OCR wordlist if needed.
3. **After config change:** upload known-safe and known-bad fixtures; confirm action (`pending` / `rejected`) and optional reject email.

## Minimal operator path

1. Enable moderation in Admin; set threshold / action.
2. If using OCR: set endpoint (see `deploy/ocr-paddle/README.md`), import lexicon (start from `docs/lexicon/core.example.txt`).
3. Open **Review** queue; approve / reject; note that pending/rejected images must not appear on public share pages.
4. Optional: outbound webhook `image.moderated` for n8n/scripts (Admin → Webhooks).

## What this is not

- Not a compliance certificate. Political / terror / CSAM policy remains your legal duty.
- Open source wordlists ≠ legal advice (`docs/lexicon/README.md`).
- Cloud scorer API keys stay in Admin settings DB — never commit them.

## Related

- Security hardening: `docs/security-hardening.md`
- Public product docs may link a short summary; this file is the engineering SSOT for the sampling path.
