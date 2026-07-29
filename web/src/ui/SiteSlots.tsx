import { useEffect, useMemo, useState } from 'react'
import type { HTMLInject, SiteAnnouncement, SiteFooter } from '../api/types'
import styles from './SiteSlots.module.css'

const DISMISS_KEY = 'imgli_announcement_dismissed'

function annFingerprint(a: SiteAnnouncement): string {
  return [a.text, a.link_url, a.link_label, a.starts_at, a.ends_at].join('\0')
}

/** 顶栏公告：可关闭（localStorage 记指纹）。 */
export function AnnouncementBar({ announcement }: { announcement?: SiteAnnouncement | null }) {
  const fp = useMemo(() => (announcement ? annFingerprint(announcement) : ''), [announcement])
  const [hidden, setHidden] = useState(false)

  useEffect(() => {
    if (!announcement?.dismissible) {
      setHidden(false)
      return
    }
    try {
      setHidden(localStorage.getItem(DISMISS_KEY) === fp)
    } catch {
      setHidden(false)
    }
  }, [announcement, fp])

  if (!announcement?.text || hidden) return null

  const dismiss = () => {
    if (!announcement.dismissible) return
    try {
      localStorage.setItem(DISMISS_KEY, fp)
    } catch {
      /* ignore */
    }
    setHidden(true)
  }

  return (
    <div className={styles.bar} role="region" aria-label="announcement">
      <div className={styles.barInner}>
        <span className={styles.barText}>{announcement.text}</span>
        {announcement.link_url && announcement.link_label ? (
          <a className={styles.barLink} href={announcement.link_url} rel="noopener noreferrer">
            {announcement.link_label}
          </a>
        ) : null}
        {announcement.dismissible ? (
          <button type="button" className={styles.barClose} onClick={dismiss} aria-label="Dismiss">
            ×
          </button>
        ) : null}
      </div>
    </div>
  )
}

/** 页脚链接组。 */
export function SiteFooter({ footer }: { footer?: SiteFooter | null }) {
  const groups = footer?.groups?.filter((g) => g.links?.length) ?? []
  if (!groups.length) return null
  return (
    <footer className={styles.footer}>
      <div className={styles.footerInner}>
        {groups.map((g, i) => (
          <div key={i} className={styles.footerGroup}>
            {g.title ? <div className={styles.footerTitle}>{g.title}</div> : null}
            <ul className={styles.footerList}>
              {g.links.map((l, j) => (
                <li key={j}>
                  <a href={l.url} rel="noopener noreferrer">
                    {l.label}
                  </a>
                </li>
              ))}
            </ul>
          </div>
        ))}
      </div>
    </footer>
  )
}

/**
 * 自定义 HTML 注入（自托管自伤面）。
 * head：解析后挂到 document.head；body_end：追加到 document.body 末尾。
 */
export function HtmlInject({ inject }: { inject?: HTMLInject | null }) {
  useEffect(() => {
    if (!inject) return
    const headNodes: Node[] = []
    const bodyNodes: Node[] = []

    if (inject.head?.trim()) {
      const tpl = document.createElement('template')
      tpl.innerHTML = inject.head
      tpl.content.childNodes.forEach((n) => {
        const c = n.cloneNode(true)
        document.head.appendChild(c)
        headNodes.push(c)
      })
    }
    if (inject.body_end?.trim()) {
      const tpl = document.createElement('template')
      tpl.innerHTML = inject.body_end
      tpl.content.childNodes.forEach((n) => {
        const c = n.cloneNode(true)
        document.body.appendChild(c)
        bodyNodes.push(c)
      })
    }

    return () => {
      headNodes.forEach((n) => n.parentNode?.removeChild(n))
      bodyNodes.forEach((n) => n.parentNode?.removeChild(n))
    }
  }, [inject?.head, inject?.body_end])

  return null
}
