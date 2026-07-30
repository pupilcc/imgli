import type { Dispatch, SetStateAction } from 'react'
import type { FooterGroup, SiteAnnouncement } from '../../../../api/types'
import { useT } from '../../../../i18n'
import { Button } from '../../../../ui/Button'
import { Input } from '../../../../ui/Input'
import { Toggle } from '../../../../ui/Toggle'
import type { FormSet, FormState } from '../settingsForm'
import styles from '../SettingsPage.module.css'

interface Props {
  form: FormState
  set: FormSet
  setForm: Dispatch<SetStateAction<FormState | null>>
  setAnn: <K extends keyof SiteAnnouncement>(k: K, v: SiteAnnouncement[K]) => void
  patchFooterGroup: (gi: number, patch: Partial<FooterGroup>) => void
  patchFooterLink: (gi: number, li: number, patch: { label?: string; url?: string }) => void
}

export function SlotsTab({ form, set, setForm, setAnn, patchFooterGroup, patchFooterLink }: Props) {
  const { t } = useT()
  return (
    <>
      <section className={styles.section}>
        <div className={styles.h2Row}>
          <h2 className={styles.h2}>{t('adminB.announcement')}</h2>
          <Toggle checked={form.ann.enabled} onChange={(v) => setAnn('enabled', v)} />
        </div>
        <span className={styles.hint}>{t('adminB.announcementHint')}</span>
        <Input
          label={t('adminB.announcementText')}
          value={form.ann.text}
          maxLength={500}
          onChange={(e) => setAnn('text', e.target.value)}
        />
        <Input
          label={t('adminB.announcementLinkUrl')}
          value={form.ann.link_url}
          placeholder="https://… or /path"
          onChange={(e) => setAnn('link_url', e.target.value)}
        />
        <Input
          label={t('adminB.announcementLinkLabel')}
          value={form.ann.link_label}
          maxLength={80}
          onChange={(e) => setAnn('link_label', e.target.value)}
        />
        <div className={styles.field}>
          <div className={styles.sliderHead}>
            <span className={styles.label}>{t('adminB.announcementDismissible')}</span>
            <Toggle
              aria-label={t('adminB.announcementDismissible')}
              checked={form.ann.dismissible}
              onChange={(v) => setAnn('dismissible', v)}
            />
          </div>
        </div>
        <Input
          label={t('adminB.announcementStarts')}
          value={form.ann.starts_at}
          placeholder="2026-07-01T00:00:00Z"
          onChange={(e) => setAnn('starts_at', e.target.value)}
        />
        <Input
          label={t('adminB.announcementEnds')}
          value={form.ann.ends_at}
          placeholder="2026-08-01T00:00:00Z"
          onChange={(e) => setAnn('ends_at', e.target.value)}
        />
      </section>

      <section className={styles.section}>
        <h2 className={styles.h2}>{t('adminB.footerLinks')}</h2>
        <span className={styles.hint}>{t('adminB.footerLinksHint')}</span>
        {form.footerGroups.map((g, gi) => (
          <div key={gi} className={styles.slotCard}>
            <Input
              label={t('adminB.footerGroupTitle')}
              value={g.title}
              maxLength={80}
              onChange={(e) => patchFooterGroup(gi, { title: e.target.value })}
            />
            {g.links.map((l, li) => (
              <div key={li} className={styles.slotRow}>
                <Input
                  label={t('adminB.footerLinkLabel')}
                  value={l.label}
                  maxLength={80}
                  onChange={(e) => patchFooterLink(gi, li, { label: e.target.value })}
                />
                <Input
                  label={t('adminB.footerLinkUrl')}
                  value={l.url}
                  onChange={(e) => patchFooterLink(gi, li, { url: e.target.value })}
                />
                <Button
                  type="button"
                  variant="ghost"
                  onClick={() =>
                    patchFooterGroup(gi, {
                      links: g.links.filter((_, j) => j !== li),
                    })
                  }
                >
                  {t('adminB.removeLink')}
                </Button>
              </div>
            ))}
            <div className={styles.slotActions}>
              <Button
                type="button"
                variant="ghost"
                onClick={() =>
                  patchFooterGroup(gi, {
                    links: [...g.links, { label: '', url: '' }],
                  })
                }
              >
                {t('adminB.addLink')}
              </Button>
              <Button
                type="button"
                variant="ghost"
                onClick={() =>
                  setForm((f) =>
                    f ? { ...f, footerGroups: f.footerGroups.filter((_, i) => i !== gi) } : f,
                  )
                }
              >
                {t('adminB.removeGroup')}
              </Button>
            </div>
          </div>
        ))}
        <Button
          type="button"
          variant="ghost"
          onClick={() =>
            setForm((f) =>
              f
                ? {
                    ...f,
                    footerGroups: [...f.footerGroups, { title: '', links: [{ label: '', url: '' }] }],
                  }
                : f,
            )
          }
        >
          {t('adminB.addGroup')}
        </Button>
      </section>

      <section className={styles.section}>
        <h2 className={styles.h2}>{t('adminB.htmlInject')}</h2>
        <span className={styles.hintWarn}>{t('adminB.htmlInjectWarn')}</span>
        <div className={styles.field}>
          <span className={styles.label}>{t('adminB.htmlHead')}</span>
          <textarea
            className={styles.textarea}
            rows={5}
            value={form.htmlHead}
            onChange={(e) => set('htmlHead', e.target.value)}
            spellCheck={false}
          />
        </div>
        <div className={styles.field}>
          <span className={styles.label}>{t('adminB.htmlBodyEnd')}</span>
          <textarea
            className={styles.textarea}
            rows={5}
            value={form.htmlBodyEnd}
            onChange={(e) => set('htmlBodyEnd', e.target.value)}
            spellCheck={false}
          />
        </div>
      </section>
    </>
  )
}
