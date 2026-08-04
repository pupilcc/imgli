import { useT } from '../../../../i18n'
import { Input } from '../../../../ui/Input'
import { Segmented } from '../../../../ui/Segmented'
import { Toggle } from '../../../../ui/Toggle'
import type { FormSet, FormState } from '../settingsForm'
import { s } from '../settingsUi'

export function BasicTab({ form, set }: { form: FormState; set: FormSet }) {
  const { t } = useT()
  return (
    <section className={s.section}>
      <h2 className={s.h2}>{t('adminB.basic')}</h2>
      <div className={s.field}>
        <Input label={t('adminB.siteName')} value={form.siteName} maxLength={64} onChange={(e) => set('siteName', e.target.value)} />
        <span className={s.hint}>{t('adminB.siteNameHint')}</span>
      </div>
      <div className={s.field}>
        <span className={s.label}>{t('adminB.regMode')}</span>
        <Segmented
          options={[
            { value: 'open', label: t('adminB.regOpen') },
            { value: 'invite', label: t('adminB.regInvite') },
            { value: 'closed', label: t('adminB.regClosed') },
          ]}
          value={form.regMode}
          onChange={(v) => set('regMode', v)}
        />
      </div>
      <div className={s.field}>
        <div className={s.sliderHead}>
          <span className={s.label}>{t('adminB.guestUpload')}</span>
          <Toggle aria-label={t('adminB.guestUpload')} checked={form.guestUpload} onChange={(v) => set('guestUpload', v)} />
        </div>
        <span className={s.hint}>{t('adminB.guestUploadHint')}</span>
      </div>
      <div className={s.field}>
        <div className={s.sliderHead}>
          <span className={s.label}>{t('adminB.plazaEnabled')}</span>
          <Toggle aria-label={t('adminB.plazaEnabled')} checked={form.plazaEnabled} onChange={(v) => set('plazaEnabled', v)} />
        </div>
        <span className={s.hint}>{t('adminB.plazaEnabledHint')}</span>
      </div>
    </section>
  )
}
