#!/usr/bin/env python3
"""One-shot: set img.li public announcement + footer slots (SQLite settings table).

Usage on the host:
  python3 ops-set-public-slots.py /path/to/baili.db
"""
from __future__ import annotations

import json
import sqlite3
import sys
from pathlib import Path


def main() -> int:
    if len(sys.argv) != 2:
        print("usage: ops-set-public-slots.py <sqlite-db>", file=sys.stderr)
        return 2
    db_path = Path(sys.argv[1])
    if not db_path.is_file():
        print(f"db not found: {db_path}", file=sys.stderr)
        return 2

    ann = {
        "enabled": True,
        "text": (
            "Public trial: ~1GB storage / 5GB monthly bandwidth — not unlimited CDN. "
            "Heavy use: self-host open-source imgli."
        ),
        "link_url": "https://docs.imgli.com/public-instance",
        "link_label": "Limits",
        "dismissible": True,
        "starts_at": "",
        "ends_at": "",
    }
    # Bilingual-friendly short Chinese primary for img.li CN audience
    ann_zh = {
        "enabled": True,
        "text": (
            "公共试用：约 1GB 存储 / 5GB 月流量，非无限 CDN。"
            "重度使用请自托管开源版 imgli。"
        ),
        "link_url": "https://docs.imgli.com/public-instance",
        "link_label": "说明",
        "dismissible": True,
        "starts_at": "",
        "ends_at": "",
    }
    footer = {
        "groups": [
            {
                "title": "产品",
                "links": [
                    {"label": "产品站", "url": "https://imgli.com"},
                    {"label": "文档", "url": "https://docs.imgli.com"},
                    {"label": "GitHub", "url": "https://github.com/yixian-huang/imgli"},
                ],
            },
            {
                "title": "试用与自托管",
                "links": [
                    {"label": "公共实例限额", "url": "https://docs.imgli.com/public-instance"},
                    {"label": "快速开始", "url": "https://docs.imgli.com/quickstart"},
                    {"label": "PicGo 对接", "url": "https://docs.imgli.com/picgo"},
                ],
            },
            {
                "title": "社区",
                "links": [
                    {"label": "Issues", "url": "https://github.com/yixian-huang/imgli/issues"},
                    {"label": "许可 / 商业", "url": "https://docs.imgli.com/commercial"},
                    {"label": "安全", "url": "https://docs.imgli.com/security"},
                ],
            },
        ]
    }

    # Prefer Chinese announcement for img.li (primary audience)
    payload = {
        "announcement": ann_zh,
        "footer": footer,
        # Public copy CTAs (operator-owned; not hard-coded in OSS UI)
        "help_url": "https://docs.imgli.com/public-instance",
        "upgrade_url": "https://docs.imgli.com/quickstart",
        "register_notice": (
            "公共试用：约 1GB 存储 / 5GB 月流量（以用户组为准），非无限 CDN。"
            "重度使用请自托管开源版。"
        ),
        "share_branding": "links",
    }

    con = sqlite3.connect(str(db_path))
    cur = con.cursor()
    for key, val in payload.items():
        if isinstance(val, (dict, list)):
            raw = json.dumps(val, ensure_ascii=False)
        else:
            raw = json.dumps(val, ensure_ascii=False)
        cur.execute("UPDATE settings SET value = ? WHERE key = ?", (raw, key))
        if cur.rowcount == 0:
            cur.execute("INSERT INTO settings(key, value) VALUES (?, ?)", (key, raw))
    con.commit()
    rows = cur.execute(
        "SELECT key, substr(value, 1, 120) FROM settings WHERE key IN ("
        "'announcement','footer','help_url','upgrade_url','register_notice','share_branding')"
    ).fetchall()
    con.close()
    print("updated:")
    for r in rows:
        print(r[0], r[1])
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
