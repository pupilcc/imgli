#!/usr/bin/env python3
"""Patch imgli.com Inkless home content_documents (draft + published).

  python3 ops-patch-imglicom-home.py /path/to/inkless.db /path/to/home.json
"""
from __future__ import annotations

import json
import sqlite3
import sys
from datetime import datetime, timezone
from pathlib import Path


def main() -> int:
    if len(sys.argv) != 3:
        print("usage: ops-patch-imglicom-home.py <inkless.db> <home.json>", file=sys.stderr)
        return 2
    db_path = Path(sys.argv[1])
    cfg_path = Path(sys.argv[2])
    if not db_path.is_file() or not cfg_path.is_file():
        print("db or json missing", file=sys.stderr)
        return 2
    cfg = json.loads(cfg_path.read_text(encoding="utf-8"))
    raw = json.dumps(cfg, ensure_ascii=False)
    now = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M:%S")
    con = sqlite3.connect(str(db_path))
    cur = con.cursor()
    cur.execute(
        """
        UPDATE content_documents
        SET draft_config = ?, published_config = ?,
            draft_version = draft_version + 1,
            published_version = published_version + 1,
            updated_at = ?
        WHERE page_key = 'home'
        """,
        (raw, raw, now),
    )
    if cur.rowcount == 0:
        cur.execute(
            """
            INSERT INTO content_documents(
              page_key, draft_config, draft_version,
              published_config, published_version, updated_at
            ) VALUES ('home', ?, 1, ?, 1, ?)
            """,
            (raw, raw, now),
        )
    con.commit()
    row = cur.execute(
        "SELECT page_key, length(published_config), published_version FROM content_documents WHERE page_key='home'"
    ).fetchone()
    con.close()
    print("ok", row)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
