import { useT } from '../../../../i18n'
import { Toggle } from '../../../../ui/Toggle'
import type { FormSet, FormState } from '../settingsForm'
import { s } from '../settingsUi'

export function HotlinkTab({ form, set }: { form: FormState; set: FormSet }) {
  const { t } = useT()
  return (
    <section className={s.section}>
      <div className={s.h2Row}>
        <h2 className={s.h2}>{t('adminB.hotlink')}</h2>
        <Toggle aria-label={t('adminB.enableHotlink')} checked={form.hotlinkEnabled} onChange={(v) => set('hotlinkEnabled', v)} />
      </div>
      <div className={s.field}>
        <label className={s.label} htmlFor="hotlink-domains">{t('adminB.allowedDomains')}</label>
        <textarea
          id="hotlink-domains"
          className={s.textarea}
          rows={4}
          placeholder={'example.com\n*.cdn.example.com'}
          value={form.hotlinkDomains}
          onChange={(e) => set('hotlinkDomains', e.target.value)}
        />
        <span className={s.hint}>{t('adminB.domainsHint')}</span>
      </div>
      <div className={s.field}>
        <div className={s.sliderHead}>
          <span className={s.label}>{t('adminB.allowEmptyReferer')}</span>
          <Toggle aria-label={t('adminB.allowEmptyReferer')} checked={form.hotlinkAllowEmpty} onChange={(v) => set('hotlinkAllowEmpty', v)} />
        </div>
        <span className={s.hint}>{t('adminB.emptyRefererHint')}</span>
      </div>
    </section>
  )
}
