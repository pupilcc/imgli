import type { RefObject } from 'react'
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
  keywordCount: number
  lexiconFileRef: RefObject<HTMLInputElement | null>
  lexiconImportMode: RefObject<'replace' | 'merge'>
  onImportFile: (file: File, mode: 'replace' | 'merge') => void
  onExport: () => void
}

export function OcrTab({
  form,
  set,
  keywordCount,
  lexiconFileRef,
  lexiconImportMode,
  onImportFile,
  onExport,
}: Props) {
  const { t } = useT()
  return (
    <section className={s.section}>
      <div className={s.h2Row}>
        <h2 className={s.h2}>{t('adminB.ocrSection')}</h2>
        <Toggle
          aria-label={t('adminB.ocrEnable')}
          checked={form.ocrEnabled}
          onChange={(v) => set('ocrEnabled', v)}
        />
      </div>
      <span className={s.hint}>{t('adminB.ocrEnableHint')}</span>
      <Input
        label={t('adminB.ocrEndpoint')}
        placeholder={t('adminB.ocrEndpointPlaceholder')}
        value={form.ocrEndpoint}
        onChange={(e) => set('ocrEndpoint', e.target.value)}
      />
      <Input
        label={t('adminB.ocrApiKey')}
        placeholder={t('adminB.noKeyPlaceholder')}
        value={form.ocrApiKey}
        extra={<span className={s.hint}>{t('adminB.secretMaskHintSettings')}</span>}
        onChange={(e) => set('ocrApiKey', e.target.value)}
        onFocus={(e) => e.target.select()}
      />
      <div className={s.field}>
        <div className={s.sliderHead}>
          <label className={s.label} htmlFor="ocr-keywords">
            {t('adminB.ocrKeywords')}
          </label>
          <span className={s.mono}>{t('adminB.ocrKeywordCount', { count: keywordCount })}</span>
        </div>
        <textarea
          id="ocr-keywords"
          className={s.textarea}
          rows={10}
          placeholder={t('adminB.ocrKeywordsPlaceholder')}
          value={form.ocrKeywords}
          onChange={(e) => set('ocrKeywords', e.target.value)}
        />
        <div className={s.lexiconToolbar}>
          <input
            ref={lexiconFileRef}
            type="file"
            accept=".txt,text/plain"
            className={s.fileInput}
            aria-hidden
            tabIndex={-1}
            onChange={(e) => {
              const f = e.target.files?.[0]
              e.target.value = ''
              if (f) onImportFile(f, lexiconImportMode.current)
            }}
          />
          <Button
            variant="secondary"
            type="button"
            onClick={() => {
              lexiconImportMode.current = 'merge'
              lexiconFileRef.current?.click()
            }}
          >
            {t('adminB.ocrImportMerge')}
          </Button>
          <Button
            variant="secondary"
            type="button"
            onClick={() => {
              lexiconImportMode.current = 'replace'
              lexiconFileRef.current?.click()
            }}
          >
            {t('adminB.ocrImportReplace')}
          </Button>
          <Button variant="secondary" type="button" onClick={onExport} disabled={keywordCount === 0}>
            {t('adminB.ocrExport')}
          </Button>
        </div>
        <span className={s.hint}>{t('adminB.ocrKeywordsHint')}</span>
        <span className={s.hint}>{t('adminB.ocrLexiconNote')}</span>
      </div>
      <div className={s.field}>
        <span className={s.label}>{t('adminB.ocrOnHit')}</span>
        <Segmented
          options={[
            { value: 'review', label: t('adminB.actionPending') },
            { value: 'block', label: t('adminB.actionRejected') },
          ]}
          value={form.ocrOnHit}
          onChange={(v) => set('ocrOnHit', v)}
        />
      </div>
    </section>
  )
}
