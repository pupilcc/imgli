import { useT } from '../i18n'
import { cn } from '../lib/cn'

type Props = {
  siteName: string
  branding?: 'off' | 'site' | 'links' | string
  helpURL?: string
  upgradeURL?: string
  className?: string
  /** 默认 share-brand-foot；相册页可用 album-share-brand-foot */
  testId?: string
}

/** 分享页 / 公开相册页脚：开源署名 + 可选站名/链接。 */
export function ShareBrandFooter({
  siteName,
  branding = 'off',
  helpURL = '',
  upgradeURL = '',
  className,
  testId = 'share-brand-foot',
}: Props) {
  const { t } = useT()
  const showBrand = branding === 'site' || branding === 'links'
  const help = helpURL.trim()
  const upgrade = upgradeURL.trim()

  return (
    <footer
      className={cn(
        'flex flex-wrap items-center gap-1.5 text-xs text-muted [&_a]:text-ink [&_a]:underline [&_a]:underline-offset-2',
        className,
      )}
      data-testid={testId}
    >
      <span>{t('share.ossCredit')}</span>
      {showBrand && (
        <>
          <span className="opacity-45">·</span>
          <span>{t('share.brandVia', { site: siteName })}</span>
        </>
      )}
      {branding === 'links' && (help || upgrade) && (
        <>
          {help && (
            <>
              <span className="opacity-45">·</span>
              <a href={help} rel="noopener noreferrer">
                {t('share.helpLink')}
              </a>
            </>
          )}
          {upgrade && (
            <>
              <span className="opacity-45">·</span>
              <a href={upgrade} rel="noopener noreferrer">
                {t('share.upgradeLink')}
              </a>
            </>
          )}
        </>
      )}
    </footer>
  )
}
