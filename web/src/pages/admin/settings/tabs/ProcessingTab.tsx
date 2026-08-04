import { POSITIONS } from '../../../../api/types'
import { useT } from '../../../../i18n'
import { Input } from '../../../../ui/Input'
import { Toggle } from '../../../../ui/Toggle'
import type { FormSet, FormState } from '../settingsForm'
import { s } from '../settingsUi'

export function ProcessingTab({ form, set }: { form: FormState; set: FormSet }) {
  const { t } = useT()
  return (
    <section className={s.section}>
      <div className={s.h2Row}>
        <h2 className={s.h2}>{t('adminB.processing')}</h2>
      </div>
      <div className={s.field}>
        <div className={s.sliderHead}>
          <span className={s.label}>{t('adminB.stripExif')}</span>
          <Toggle aria-label={t('adminB.stripExif')} checked={form.stripExif} onChange={(v) => set('stripExif', v)} />
        </div>
        <span className={s.hint}>{t('adminB.stripExifHint')}</span>
      </div>
      <div className={s.h2Row}>
        <h3 className={s.h2}>{t('adminB.textWatermark')}</h3>
        <Toggle aria-label={t('adminB.enableTextWatermark')} checked={form.twEnabled} onChange={(v) => set('twEnabled', v)} />
      </div>
      <Input
        label={t('adminB.textWatermark')}
        placeholder={t('adminB.textWatermarkPlaceholder')}
        value={form.twText}
        onChange={(e) => set('twText', e.target.value)}
      />
      <div className={s.field}>
        <label className={s.label} htmlFor="proc-wm-pos">
          {t('adminB.watermarkPos')}
        </label>
        <select
          id="proc-wm-pos"
          className={s.select}
          value={form.twPos}
          onChange={(e) => set('twPos', e.target.value)}
        >
          {POSITIONS.map((pos) => (
            <option key={pos.value} value={pos.value}>
              {t(pos.label)}
            </option>
          ))}
        </select>
      </div>
      <div className={s.field}>
        <div className={s.sliderHead}>
          <span className={s.label}>{t('adminB.opacity')}</span>
          <span className={s.mono}>{form.twOpacity.toFixed(2)}</span>
        </div>
        <input
          className={s.slider}
          type="range"
          min={0.05}
          max={1}
          step={0.05}
          value={form.twOpacity}
          aria-label={t('adminB.textOpacityAria')}
          onChange={(e) => set('twOpacity', Number(e.target.value))}
        />
      </div>
      <div className={s.field}>
        <div className={s.sliderHead}>
          <span className={s.label}>{t('adminB.sizeRatio')}</span>
          <span className={s.mono}>{form.twSizeRatio.toFixed(2)}</span>
        </div>
        <input
          className={s.slider}
          type="range"
          min={0.01}
          max={0.2}
          step={0.01}
          value={form.twSizeRatio}
          aria-label={t('adminB.sizeRatio')}
          onChange={(e) => set('twSizeRatio', Number(e.target.value))}
        />
      </div>
      <Input
        label={t('adminB.maxEdge')}
        type="number"
        min={0}
        max={16384}
        value={String(form.maxEdge)}
        onChange={(e) => set('maxEdge', Number(e.target.value) || 0)}
        extra={<span className={s.hint}>{t('adminB.maxEdgeHint')}</span>}
      />
    </section>
  )
}
