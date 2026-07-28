import { useEffect } from 'react'
import { Link } from 'react-router'
import type { DiscoverRow } from '../../api/types'
import { useT } from '../../i18n'
import { copyText } from '../../lib/copy'
import styles from './Lightbox.module.css'

interface Props {
  row: DiscoverRow | null
  onClose: () => void
}

/** 广场灯箱：大图预览 + 复制外链 + 作者入口。row 为 null 时不渲染。 */
export function Lightbox({ row, onClose }: Props) {
  const { t } = useT()

  useEffect(() => {
    if (!row) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [row, onClose])

  if (!row) return null

  const displayName = row.author.nickname || row.author.username
  const externalUrl = `${location.origin}/i/${row.key}`

  return (
    <div className={styles.mask} onClick={onClose} role="presentation">
      <div
        className={styles.panel}
        role="dialog"
        aria-modal="true"
        aria-label={row.name}
        onClick={(e) => e.stopPropagation()}
      >
        <button type="button" className={styles.closeBtn} title={t('discover.close')} onClick={onClose}>
          ×
        </button>
        <img
          className={styles.image}
          src={`/i/${row.key}`}
          alt={row.name}
          onClick={(e) => e.stopPropagation()}
        />
        <div className={styles.footer}>
          <Link to={`/u/${row.author.username}`} className={styles.authorLink} onClick={onClose}>
            {displayName}
          </Link>
          <button
            type="button"
            className={styles.copyBtn}
            onClick={() => copyText(externalUrl, t('discover.externalLink'))}
          >
            {t('discover.copyExternal')}
          </button>
        </div>
      </div>
    </div>
  )
}
