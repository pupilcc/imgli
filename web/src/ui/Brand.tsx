import type { CSSProperties } from 'react'
import styles from './Brand.module.css'

/** 产品字标（固定小写 img.li，与域名/品牌「图鲤」一致）。 */
export const BRAND_WORDMARK = 'img.li'
export const BRAND_CN = '图鲤'

type MarkProps = {
  /** 图形高度（px）；宽度按 viewBox 80×100 等比。导航默认 20。 */
  size?: number
  className?: string
  title?: string
}

/**
 * 图鲤主标（几何白鲤）。fill 使用 currentColor，跟随主题/反色面板。
 * 鱼眼镂空（evenodd），任意底色可读。
 */
export function BrandMark({ size = 20, className, title }: MarkProps) {
  const h = size
  const w = (size * 80) / 100
  return (
    <svg
      className={className}
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 80 100"
      width={w}
      height={h}
      fill="currentColor"
      role={title ? 'img' : 'presentation'}
      aria-label={title}
      aria-hidden={title ? undefined : true}
    >
      {/* splash / 域名中点 */}
      <rect x="62" y="8" width="12" height="12" rx="2" />
      {/* body + hollow eye */}
      <path fillRule="evenodd" d="M44 30 L74 60 L44 90 L14 60 Z M38 52 h8 v8 h-8 Z" />
      {/* tail */}
      <path d="M14 60 L2 48 L6 60 L2 72 Z" />
    </svg>
  )
}

type LockupProps = {
  /** 图形高度，默认 20（导航规范） */
  markSize?: number
  /** 是否显示 BETA 描边徽章 */
  beta?: boolean
  /** 自定义右侧徽章文案（如 ADMIN / GUEST），优先于 beta */
  badge?: string
  /** 反色底（登录左栏等）：中点/徽章跟 currentColor 半透明，不用 --muted */
  invert?: boolean
  className?: string
  /** 字标额外样式 */
  wordClassName?: string
  style?: CSSProperties
}

/**
 * 横向锁标：鲤鱼 + 等宽 img.li（中点 muted）+ 可选徽章。
 * 颜色全部 currentColor / --muted，适配浅底与反色 brand 面板。
 */
export function BrandLockup({
  markSize = 20,
  beta,
  badge,
  invert,
  className,
  wordClassName,
  style,
}: LockupProps) {
  const tag = badge ?? (beta ? 'BETA' : undefined)
  return (
    <span
      className={[styles.lockup, invert && styles.invert, className].filter(Boolean).join(' ')}
      style={style}
    >
      <BrandMark size={markSize} className={styles.mark} />
      <span className={[styles.word, wordClassName].filter(Boolean).join(' ')}>
        img<span className={styles.dot}>.</span>li
      </span>
      {tag ? <span className={styles.badge}>{tag}</span> : null}
    </span>
  )
}
