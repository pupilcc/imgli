import { useT } from '../../../../i18n'
import { Toggle } from '../../../../ui/Toggle'
import type { FormSet, FormState } from '../settingsForm'
import styles from '../SettingsPage.module.css'

export function HotlinkTab({ form, set }: { form: FormState; set: FormSet }) {
  const { t } = useT()
  return (
    <section className={styles.section}>
      <div className={styles.h2Row}>
        <h2 className={styles.h2}>{t('adminB.hotlink')}</h2>
        <Toggle aria-label={t('adminB.enableHotlink')} checked={form.hotlinkEnabled} onChange={(v) => set('hotlinkEnabled', v)} />
      </div>
      <div className={styles.field}>
        <label className={styles.label} htmlFor="hotlink-domains">{t('adminB.allowedDomains')}</label>
        <textarea
          id="hotlink-domains"
          className={styles.textarea}
          rows={4}
          placeholder={'example.com\n*.cdn.example.com'}
          value={form.hotlinkDomains}
          onChange={(e) => set('hotlinkDomains', e.target.value)}
        />
        <span className={styles.hint}>{t('adminB.domainsHint')}</span>
      </div>
      <div className={styles.field}>
        <div className={styles.sliderHead}>
          <span className={styles.label}>{t('adminB.allowEmptyReferer')}</span>
          <Toggle aria-label={t('adminB.allowEmptyReferer')} checked={form.hotlinkAllowEmpty} onChange={(v) => set('hotlinkAllowEmpty', v)} />
        </div>
        <span className={styles.hint}>{t('adminB.emptyRefererHint')}</span>
      </div>
    </section>
  )
}
