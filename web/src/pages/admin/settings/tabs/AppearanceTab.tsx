import { useT } from '../../../../i18n'
import { contrastOnAccent, normalizeAccent } from '../../../../lib/siteTheme'
import { Input } from '../../../../ui/Input'
import type { FormSet, FormState } from '../settingsForm'
import { s } from '../settingsUi'

export function AppearanceTab({ form, set }: { form: FormState; set: FormSet }) {
  const { t } = useT()
  const accent = normalizeAccent(form.themeAccent)
  const previewBtn = accent || 'var(--btn)'
  const previewText = accent ? contrastOnAccent(accent) : 'var(--btnText)'

  return (
    <section className={s.section}>
      <h2 className={s.h2}>{t('adminB.appearance')}</h2>
      <p className={s.hint}>{t('adminB.appearanceHint')}</p>

      <div className={s.field}>
        <Input
          label={t('adminB.themeAccent')}
          value={form.themeAccent}
          placeholder="#3b82f6"
          maxLength={7}
          onChange={(e) => set('themeAccent', e.target.value)}
        />
        <span className={s.hint}>{t('adminB.themeAccentHint')}</span>
        <div className="mt-2 flex flex-wrap items-center gap-2">
          <input
            type="color"
            aria-label={t('adminB.themeAccent')}
            className="h-9 w-12 cursor-pointer rounded-sm border border-border bg-surface p-0.5"
            value={accent || '#17171a'}
            onChange={(e) => set('themeAccent', e.target.value)}
          />
          <button
            type="button"
            className="cursor-pointer rounded-sm border-0 px-4 py-2 text-sm-plus font-bold"
            style={{ background: previewBtn, color: previewText }}
          >
            {t('adminB.themeAccentPreview')}
          </button>
          {form.themeAccent.trim() && (
            <button
              type="button"
              className="cursor-pointer border-0 bg-transparent text-xs font-semibold text-muted underline hover:text-ink"
              onClick={() => set('themeAccent', '')}
            >
              {t('adminB.themeAccentClear')}
            </button>
          )}
        </div>
      </div>

      <div className={s.field}>
        <Input
          label={t('adminB.themeBgImage')}
          value={form.themeBgImageUrl}
          placeholder="https://…/bg.jpg"
          onChange={(e) => set('themeBgImageUrl', e.target.value)}
        />
        <span className={s.hint}>{t('adminB.themeBgImageHint')}</span>
        {form.themeBgImageUrl.trim() && (
          <div
            className="mt-2 h-24 overflow-hidden rounded-sm border border-border bg-soft bg-cover bg-center"
            style={{
              backgroundImage: `url("${form.themeBgImageUrl.trim().replace(/\\/g, '\\\\').replace(/"/g, '\\"')}")`,
            }}
            role="img"
            aria-label={t('adminB.themeBgImage')}
          />
        )}
      </div>

      <div className={s.field}>
        <div className={s.sliderHead}>
          <span className={s.label}>{t('adminB.themeBgDim')}</span>
          <span className="font-mono text-xs text-muted tabular-nums">{form.themeBgDim.toFixed(2)}</span>
        </div>
        <input
          type="range"
          min={0}
          max={1}
          step={0.01}
          value={form.themeBgDim}
          disabled={!form.themeBgImageUrl.trim()}
          onChange={(e) => set('themeBgDim', Number(e.target.value))}
          className="w-full accent-[var(--btn)]"
          aria-label={t('adminB.themeBgDim')}
        />
        <span className={s.hint}>{t('adminB.themeBgDimHint')}</span>
      </div>

      <div className={s.field}>
        <div className={s.sliderHead}>
          <span className={s.label}>{t('adminB.themeGlass')}</span>
          <span className="font-mono text-xs text-muted tabular-nums">{form.themeGlass.toFixed(2)}</span>
        </div>
        <input
          type="range"
          min={0}
          max={1}
          step={0.01}
          value={form.themeGlass}
          disabled={!form.themeBgImageUrl.trim()}
          onChange={(e) => set('themeGlass', Number(e.target.value))}
          className="w-full accent-[var(--btn)]"
          aria-label={t('adminB.themeGlass')}
        />
        <span className={s.hint}>{t('adminB.themeGlassHint')}</span>
      </div>
    </section>
  )
}
