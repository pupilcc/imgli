import { POSITIONS } from '../../../../api/types'
import { useT } from '../../../../i18n'
import { Input } from '../../../../ui/Input'
import { Toggle } from '../../../../ui/Toggle'
import type { FormSet, FormState } from '../settingsForm'
import styles from '../SettingsPage.module.css'

export function ProcessingTab({ form, set }: { form: FormState; set: FormSet }) {
  const { t } = useT()
  return (
    <section className={styles.section}>
      <div className={styles.h2Row}>
        <h2 className={styles.h2}>{t('adminB.processing')}</h2>
      </div>
      <div className={styles.field}>
        <div className={styles.sliderHead}>
          <span className={styles.label}>{t('adminB.stripExif')}</span>
          <Toggle aria-label={t('adminB.stripExif')} checked={form.stripExif} onChange={(v) => set('stripExif', v)} />
        </div>
        <span className={styles.hint}>{t('adminB.stripExifHint')}</span>
      </div>
      <div className={styles.h2Row}>
        <h3 className={styles.h2}>{t('adminB.textWatermark')}</h3>
        <Toggle aria-label={t('adminB.enableTextWatermark')} checked={form.twEnabled} onChange={(v) => set('twEnabled', v)} />
      </div>
      <Input
        label={t('adminB.textWatermark')}
        placeholder={t('adminB.textWatermarkPlaceholder')}
        value={form.twText}
        onChange={(e) => set('twText', e.target.value)}
      />
      <div className={styles.field}>
        <label className={styles.label} htmlFor="proc-wm-pos">
          {t('adminB.watermarkPos')}
        </label>
        <select
          id="proc-wm-pos"
          className={styles.select}
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
      <div className={styles.field}>
        <div className={styles.sliderHead}>
          <span className={styles.label}>{t('adminB.opacity')}</span>
          <span className={styles.mono}>{form.twOpacity.toFixed(2)}</span>
        </div>
        <input
          className={styles.slider}
          type="range"
          min={0.05}
          max={1}
          step={0.05}
          value={form.twOpacity}
          aria-label={t('adminB.textOpacityAria')}
          onChange={(e) => set('twOpacity', Number(e.target.value))}
        />
      </div>
      <div className={styles.field}>
        <div className={styles.sliderHead}>
          <span className={styles.label}>{t('adminB.sizeRatio')}</span>
          <span className={styles.mono}>{form.twSizeRatio.toFixed(2)}</span>
        </div>
        <input
          className={styles.slider}
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
        extra={<span className={styles.hint}>{t('adminB.maxEdgeHint')}</span>}
      />
    </section>
  )
}
