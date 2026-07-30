import { useEffect, useMemo, useState } from 'react'
import type { HTMLInject, SiteAnnouncement, SiteFooter as SiteFooterConfig } from '../api/types'
import { useT } from '../i18n'
import { localeFingerprint, pickLocale } from '../lib/locale'
import { BRAND_WORDMARK } from './Brand'
import styles from './SiteSlots.module.css'

const DISMISS_KEY = 'imgli_announcement_dismissed'

function annFingerprint(a: SiteAnnouncement): string {
  return [
    localeFingerprint(a.text),
    a.link_url,
    localeFingerprint(a.link_label),
    a.starts_at,
    a.ends_at,
  ].join('\0')
}

/** 顶栏公告：可关闭（localStorage 记指纹）。视觉与 Nav / Quota 条对齐。 */
export function AnnouncementBar({ announcement }: { announcement?: SiteAnnouncement | null }) {
  const { t, lang } = useT()
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

  const text = announcement ? pickLocale(announcement.text, lang) : ''
  const linkLabel = announcement ? pickLocale(announcement.link_label, lang) : ''

  if (!announcement || !text || hidden) return null

  const linkURL = announcement.link_url
  const canDismiss = announcement.dismissible

  const dismiss = () => {
    if (!canDismiss) return
    try {
      localStorage.setItem(DISMISS_KEY, fp)
    } catch {
      /* ignore */
    }
    setHidden(true)
  }

  return (
    <div className={styles.bar} role="region" aria-label={t('common.announcementAria')}>
      <div className={styles.barInner}>
        <span className={styles.barKicker}>{t('common.announcementKicker')}</span>
        <span className={styles.barText}>{text}</span>
        <div className={styles.barActions}>
          {linkURL && linkLabel ? (
            <a className={styles.barLink} href={linkURL} rel="noopener noreferrer">
              {linkLabel}
            </a>
          ) : null}
          {canDismiss ? (
            <button
              type="button"
              className={styles.barClose}
              onClick={dismiss}
              aria-label={t('common.close')}
              title={t('common.close')}
            >
              ×
            </button>
          ) : null}
        </div>
      </div>
    </div>
  )
}

type SiteFooterProps = {
  footer?: SiteFooterConfig | null
  /** Instance site_name; falls back to product wordmark. */
  siteName?: string | null
}

/**
 * 页脚：以链接分组为主；底部一行克制署名。
 * 不重复砸站名/img.li/图鲤（顶栏已有品牌，此处避免标题轰炸）。
 */
export function SiteFooter({ footer, siteName }: SiteFooterProps) {
  const { t, lang } = useT()
  const groups = footer?.groups?.filter((g) => g.links?.length) ?? []
  const name = (siteName || '').trim() || BRAND_WORDMARK
  const year = new Date().getFullYear()
  // 站名已含 img.li / 图鲤 时不再叠开源工程名
  const nameLower = name.toLowerCase()
  const showOssSuffix =
    !nameLower.includes('img.li') && !nameLower.includes('imgli') && !name.includes('图鲤')

  if (groups.length === 0) {
    return (
      <footer className={`${styles.footer} ${styles.footerMinimal}`}>
        <div className={styles.footerInner}>
          <div className={styles.footerMeta}>
            <span>
              © {year} {name}
              {showOssSuffix ? ` · ${t('common.footerOss')}` : ''}
            </span>
          </div>
        </div>
      </footer>
    )
  }

  return (
    <footer className={styles.footer}>
      <div className={styles.footerInner}>
        <div className={styles.footerGroups}>
          {groups.map((g, i) => {
            const title = pickLocale(g.title, lang)
            return (
              <div key={i} className={styles.footerGroup}>
                {title ? <div className={styles.footerTitle}>{title}</div> : null}
                <ul className={styles.footerList}>
                  {g.links.map((l, j) => {
                    const label = pickLocale(l.label, lang)
                    if (!label || !l.url) return null
                    return (
                      <li key={j}>
                        <a href={l.url} rel="noopener noreferrer">
                          {label}
                        </a>
                      </li>
                    )
                  })}
                </ul>
              </div>
            )
          })}
        </div>
        <div className={styles.footerMeta}>
          <span>
            © {year} {name}
            {showOssSuffix ? ` · ${t('common.footerOss')}` : ''}
          </span>
          <span className={styles.footerMetaQuiet}>{t('meta.tagline')}</span>
        </div>
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
