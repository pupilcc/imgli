import { useT } from '../../../../i18n'
import { Button } from '../../../../ui/Button'
import { Input } from '../../../../ui/Input'
import { Segmented } from '../../../../ui/Segmented'
import { Toggle } from '../../../../ui/Toggle'
import type { FormSet, FormState } from '../settingsForm'
import styles from '../SettingsPage.module.css'

interface Props {
  form: FormState
  set: FormSet
  testPending: boolean
  onTest: () => void
}

export function ModerationTab({ form, set, testPending, onTest }: Props) {
  const { t } = useT()
  return (
    <section className={styles.section}>
      <div className={styles.h2Row}>
        <h2 className={styles.h2}>{t('adminB.moderation')}</h2>
        <Toggle checked={form.modEnabled} onChange={(v) => set('modEnabled', v)} />
      </div>
      <div className={styles.field}>
        <span className={styles.label}>{t('adminB.provider')}</span>
        <Segmented
          options={[
            { value: 'webhook', label: 'Webhook' },
            { value: 'aliyun', label: t('adminB.providerAliyun') },
            { value: 'tencent', label: t('adminB.providerTencent') },
            { value: 'openai', label: 'OpenAI' },
            { value: 'nsfwjs', label: 'NSFWJS' },
          ]}
          value={form.modProvider}
          onChange={(v) => set('modProvider', v)}
        />
      </div>
      {(form.modProvider === 'webhook' || form.modProvider === 'nsfwjs') && (
        <Input
          label={t('adminB.webhookUrl')}
          placeholder="https://..."
          value={form.modEndpoint}
          onChange={(e) => set('modEndpoint', e.target.value)}
        />
      )}
      {(form.modProvider === 'webhook' || form.modProvider === 'nsfwjs' || form.modProvider === 'openai') && (
        <Input
          label="API Key"
          placeholder={t('adminB.noKeyPlaceholder')}
          value={form.modApiKey}
          extra={<span className={styles.hint}>{t('adminB.secretMaskHintSettings')}</span>}
          onChange={(e) => set('modApiKey', e.target.value)}
          onFocus={(e) => e.target.select()}
        />
      )}
      {(form.modProvider === 'aliyun' || form.modProvider === 'tencent') && (
        <>
          <Input
            label="AccessKey ID"
            placeholder="AccessKey ID"
            value={form.modAKID}
            onChange={(e) => set('modAKID', e.target.value)}
          />
          <Input
            label="AccessKey Secret"
            placeholder={t('adminB.noKeyPlaceholder')}
            value={form.modAKSecret}
            extra={<span className={styles.hint}>{t('adminB.secretMaskHintSettings')}</span>}
            onChange={(e) => set('modAKSecret', e.target.value)}
            onFocus={(e) => e.target.select()}
          />
          <Input
            label="Region"
            placeholder={form.modProvider === 'aliyun' ? 'cn-shanghai' : 'ap-guangzhou'}
            value={form.modRegion}
            onChange={(e) => set('modRegion', e.target.value)}
          />
        </>
      )}
      <div className={styles.field}>
        <div className={styles.sliderHead}>
          <span className={styles.label}>{t('adminB.threshold')}</span>
          <span className={styles.mono}>{form.modThreshold.toFixed(2)}</span>
        </div>
        <input
          className={styles.slider}
          type="range"
          min={0}
          max={1}
          step={0.01}
          value={form.modThreshold}
          aria-label={t('adminB.threshold')}
          onChange={(e) => set('modThreshold', Number(e.target.value))}
        />
      </div>
      <div className={styles.field}>
        <span className={styles.label}>{t('adminB.overThresholdAction')}</span>
        <Segmented
          options={[
            { value: 'pending', label: t('adminB.actionPending') },
            { value: 'rejected', label: t('adminB.actionRejected') },
          ]}
          value={form.modAction}
          onChange={(v) => set('modAction', v)}
        />
      </div>
      <div className={styles.field}>
        <div className={styles.sliderHead}>
          <span className={styles.label}>{t('adminB.loginSampleRate')}</span>
          <span className={styles.mono}>{(form.loginSampleRate * 100).toFixed(0)}%</span>
        </div>
        <input
          className={styles.slider}
          type="range"
          min={0}
          max={1}
          step={0.05}
          value={form.loginSampleRate}
          aria-label={t('adminB.loginSampleRate')}
          onChange={(e) => set('loginSampleRate', Number(e.target.value))}
        />
        <span className={styles.hint}>{t('adminB.loginSampleRateHint')}</span>
      </div>
      <div className={styles.field}>
        <span className={styles.label}>{t('adminB.onPluginError')}</span>
        <Segmented
          options={[
            { value: 'open', label: t('adminB.onPluginErrorOpen') },
            { value: 'review', label: t('adminB.onPluginErrorReview') },
          ]}
          value={form.onPluginError}
          onChange={(v) => set('onPluginError', v)}
        />
        <span className={styles.hint}>{t('adminB.onPluginErrorHint')}</span>
      </div>
      <div className={styles.row}>
        <span className={styles.label}>{t('adminB.notifyOnReject')}</span>
        <Toggle
          aria-label={t('adminB.notifyOnReject')}
          checked={form.notifyOnReject}
          onChange={(v) => set('notifyOnReject', v)}
        />
      </div>
      <span className={styles.hint}>{t('adminB.notifyOnRejectHint')}</span>
      <div className={styles.field}>
        <Button variant="secondary" disabled={testPending} onClick={onTest}>
          {t('adminB.testModeration')}
        </Button>
      </div>
    </section>
  )
}
