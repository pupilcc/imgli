import { useT } from '../../../../i18n'
import { Button } from '../../../../ui/Button'
import { Input } from '../../../../ui/Input'
import { Segmented } from '../../../../ui/Segmented'
import { Toggle } from '../../../../ui/Toggle'
import type { FormSet, FormState } from '../settingsForm'
import { s } from '../settingsUi'

interface Props {
  form: FormState
  set: FormSet
  testTo: string
  setTestTo: (v: string) => void
  testMsg: string | null
  testPending: boolean
  onTest: () => void
}

export function SmtpTab({ form, set, testTo, setTestTo, testMsg, testPending, onTest }: Props) {
  const { t } = useT()
  return (
    <section className={s.section}>
      <h2 className={s.h2}>{t('adminB.smtpSection')}</h2>
      <Input
        label={t('adminB.smtpHost')}
        placeholder={t('adminB.smtpHostPlaceholder')}
        value={form.smtpHost}
        onChange={(e) => set('smtpHost', e.target.value)}
      />
      <Input
        label={t('adminB.port')}
        type="number"
        value={String(form.smtpPort)}
        onChange={(e) => set('smtpPort', Number(e.target.value) || 0)}
      />
      <Input
        label={t('adminB.username')}
        placeholder={t('adminB.noAuthPlaceholder')}
        value={form.smtpUser}
        onChange={(e) => set('smtpUser', e.target.value)}
      />
      <Input
        label={t('adminB.smtpPassword')}
        placeholder={t('adminB.noPasswordPlaceholder')}
        value={form.smtpPassword}
        extra={<span className={s.hint}>{t('adminB.passwordMaskHint')}</span>}
        onChange={(e) => set('smtpPassword', e.target.value)}
        onFocus={(e) => e.target.select()}
      />
      <Input
        label={t('adminB.from')}
        placeholder="no-reply@img.li"
        value={form.smtpFrom}
        onChange={(e) => set('smtpFrom', e.target.value)}
      />
      <div className={s.field}>
        <span className={s.label}>{t('adminB.encryption')}</span>
        <Segmented
          options={[
            { value: 'none', label: t('adminB.noEncryption') },
            { value: 'starttls', label: 'STARTTLS' },
            { value: 'ssl', label: 'SSL' },
          ]}
          value={form.smtpEnc}
          onChange={(v) => set('smtpEnc', v)}
        />
      </div>
      <div className={s.field}>
        <div className={s.sliderHead}>
          <span className={s.label}>{t('adminB.welcomeEmail')}</span>
          <Toggle
            aria-label={t('adminB.welcomeEmail')}
            checked={form.welcomeEmail}
            onChange={(v) => set('welcomeEmail', v)}
          />
        </div>
        <span className={s.hint}>{t('adminB.welcomeEmailHint')}</span>
      </div>
      <div className={s.field}>
        <span className={s.label}>{t('adminB.testSend')}</span>
        <div className={s.testRow}>
          <Input
            label={t('adminB.testRecipient')}
            placeholder="you@example.com"
            value={testTo}
            onChange={(e) => setTestTo(e.target.value)}
          />
          <Button variant="secondary" disabled={testPending} onClick={onTest}>
            {t('adminB.sendTestEmail')}
          </Button>
        </div>
        {testMsg && <span className={s.hint}>{testMsg}</span>}
      </div>
    </section>
  )
}
