import { useEffect, useState } from 'react'
import { Link } from 'react-router'
import type { DiscoverRow } from '../../api/types'
import styles from './ImageCard.module.css'

interface Props {
  row: DiscoverRow
  onOpen: (row: DiscoverRow) => void
}

/** 广场/用户图库卡片：缩略图 + 作者行，点击整卡打开灯箱。 */
export function ImageCard({ row, onOpen }: Props) {
  const [imgFailed, setImgFailed] = useState(false)
  const [avatarFailed, setAvatarFailed] = useState(false)
  const displayName = row.author.nickname || row.author.username
  const initial = (displayName[0] || '?').toUpperCase()

  useEffect(() => {
    setImgFailed(false)
  }, [row.key])

  useEffect(() => {
    setAvatarFailed(false)
  }, [row.author.user_id, row.author.avatar_version])

  return (
    <div
      className={styles.card}
      role="button"
      tabIndex={0}
      onClick={() => onOpen(row)}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault()
          onOpen(row)
        }
      }}
    >
      <div className={styles.thumbBox}>
        {imgFailed ? (
          <div className={styles.thumbPlaceholder} aria-hidden>
            {row.ext?.toUpperCase() || 'IMG'}
          </div>
        ) : (
          <img
            className={styles.thumb}
            src={`/t/${row.key}`}
            alt={row.name}
            loading="lazy"
            onError={() => setImgFailed(true)}
          />
        )}
      </div>
      <div className={styles.author}>
        <Link
          to={`/u/${row.author.username}`}
          className={styles.authorLink}
          onClick={(e) => e.stopPropagation()}
        >
          {avatarFailed ? (
            <span className={styles.avatarFallback} aria-hidden>
              {initial}
            </span>
          ) : (
            <img
              className={styles.avatar}
              src={`/avatar/${row.author.user_id}?v=${row.author.avatar_version}`}
              alt=""
              loading="lazy"
              onError={() => setAvatarFailed(true)}
            />
          )}
          <span className={styles.name}>{displayName}</span>
        </Link>
      </div>
    </div>
  )
}
