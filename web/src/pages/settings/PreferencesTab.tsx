import { useRef, useState } from 'react'
import {
  useAlbums,
  useDeleteWatermark,
  useSession,
  useUpdatePreferences,
  useUploadWatermark,
  useUserPolicies,
} from '../../api/hooks'
import { POSITIONS, type Preferences } from '../../api/types'
import { useT } from '../../i18n'
import { useGlobal } from '../../store'
import { Button } from '../../ui/Button'
import { InlineConfirm } from '../../ui/InlineConfirm'
import { Segmented } from '../../ui/Segmented'
import { Tag } from '../../ui/Tag'
import { Toggle } from '../../ui/Toggle'
import styles from './SettingsPage.module.css'

export function PreferencesTab() {
  const { t, lang } = useT()
  const { data: user } = useSession()
  const albums = useAlbums()
  const policies = useUserPolicies()
  const save = useUpdatePreferences()
  const uploadWm = useUploadWatermark()
  const deleteWm = useDeleteWatermark()
  const pushToast = useGlobal((s) => s.pushToast)
  const fileRef = useRef<HTMLInputElement>(null)
  const p = user?.preferences
  const [albumId, setAlbumId] = useState<number | null>(p?.default_album_id ?? null)
  const [visibility, setVisibility] = useState<'public' | 'private'>(
    p?.default_visibility === 'private' ? 'private' : 'public',
  )
  const [policyId, setPolicyId] = useState<number | null>(p?.default_policy_id ?? null)
  const [copyFmt, setCopyFmt] = useState<Preferences['auto_copy_format']>(p?.auto_copy_format ?? '')
  const [wmEnabled, setWmEnabled] = useState(p?.watermark?.enabled ?? false)
  const [wmPos, setWmPos] = useState(p?.watermark?.position || 'br')
  const [wmOpacity, setWmOpacity] = useState(() => {
    const o = p?.watermark?.opacity
    return o != null && o >= 0.1 ? o : 0.5
  })
  const [wmMargin, setWmMargin] = useState(p?.watermark?.margin ?? 10)

  if (!user) return null
  const showPolicy = (policies.data?.length ?? 0) > 1

  function submit() {
    // 偏好 PATCH 全量替换:必须带上当前 lang,防清空已存 Preferences.Lang
    save.mutate(
      {
        default_album_id: albumId,
        default_visibility: visibility,
        // 策略列表未就绪时保留库中原值,防把已设偏好静默清空(codex 终审)
        default_policy_id: policies.data ? (showPolicy ? policyId : null) : (p?.default_policy_id ?? null),
        auto_copy_format: copyFmt,
        watermark: {
          enabled: wmEnabled,
          position: wmPos,
          opacity: wmOpacity,
          margin: wmMargin,
        },
        lang,
      },
      { onSuccess: () => pushToast(t('settings.toastPreferencesSaved')) },
    )
  }

  return (
    <div>
      <div className={styles.section}>
        <div className={styles.kicker}>{t('settings.preferencesKicker')}</div>
        <div className={styles.card}>
          <div className={styles.field}>
            <label className={styles.label} htmlFor="pref-album">
              {t('settings.defaultAlbum')}
            </label>
            <select
              id="pref-album"
              className={styles.input}
              value={albumId ?? 'none'}
              onChange={(e) => setAlbumId(e.target.value === 'none' ? null : Number(e.target.value))}
            >
              <option value="none">{t('settings.noAlbum')}</option>
              {albums.data?.items.map((a) => (
                <option key={a.id} value={a.id}>
                  {a.name}
                </option>
              ))}
            </select>
          </div>

          <div className={styles.field}>
            <span className={styles.label}>{t('settings.defaultVisibility')}</span>
            <Segmented<'public' | 'private'>
              options={[
                { value: 'public', label: t('settings.public') },
                { value: 'private', label: t('settings.private') },
              ]}
              value={visibility}
              onChange={setVisibility}
            />
          </div>

          {showPolicy && (
            <div className={styles.field}>
              <label className={styles.label} htmlFor="pref-policy">
                {t('settings.defaultPolicy')}
              </label>
              <select
                id="pref-policy"
                className={styles.input}
                value={policyId ?? 'none'}
                onChange={(e) => setPolicyId(e.target.value === 'none' ? null : Number(e.target.value))}
              >
                <option value="none">{t('settings.followGroupDefault')}</option>
                {policies.data?.map((pol) => (
                  <option key={pol.id} value={pol.id}>
                    {pol.name}
                  </option>
                ))}
              </select>
            </div>
          )}

          <div className={styles.field}>
            <span className={styles.label}>{t('settings.autoCopy')}</span>
            <Segmented<Preferences['auto_copy_format']>
              options={[
                { value: '', label: t('settings.autoCopyOff') },
                { value: 'url', label: 'URL' },
                { value: 'markdown', label: 'Markdown' },
                { value: 'html', label: 'HTML' },
                { value: 'bbcode', label: 'BBCode' },
                { value: 'share', label: t('settings.autoCopyShare') },
              ]}
              value={copyFmt}
              onChange={setCopyFmt}
            />
          </div>

          <div className={styles.field}>
            <span className={styles.label}>{t('settings.watermarkImage')}</span>
            <div className={styles.avatarActions}>
              {user.watermark_set ? (
                <>
                  <Tag variant="ok">{t('settings.watermarkUploaded')}</Tag>
                  <InlineConfirm
                    label={t('settings.remove')}
                    confirmLabel={t('settings.confirmRemove')}
                    onConfirm={() => deleteWm.mutate()}
                  />
                </>
              ) : (
                <>
                  <input
                    ref={fileRef}
                    type="file"
                    accept="image/png"
                    hidden
                    onChange={(e) => {
                      const f = e.target.files?.[0]
                      if (f) uploadWm.mutate(f, { onSuccess: () => pushToast(t('settings.toastWatermarkUploaded')) })
                      e.target.value = ''
                    }}
                  />
                  <Button
                    variant="secondary"
                    disabled={uploadWm.isPending}
                    onClick={() => fileRef.current?.click()}
                  >
                    {t('settings.uploadWatermark')}
                  </Button>
                </>
              )}
            </div>
          </div>

          <div className={styles.field}>
            <div className={styles.toggleRow}>
              <span className={styles.label}>{t('settings.enableWatermark')}</span>
              <Toggle aria-label={t('settings.enableWatermark')} checked={wmEnabled} onChange={setWmEnabled} />
            </div>
          </div>

          <div className={styles.field}>
            <label className={styles.label} htmlFor="pref-wm-pos">
              {t('settings.watermarkPosition')}
            </label>
            <select
              id="pref-wm-pos"
              className={styles.input}
              value={wmPos}
              onChange={(e) => setWmPos(e.target.value)}
            >
              {POSITIONS.map((pos) => (
                <option key={pos.value} value={pos.value}>
                  {t(pos.label)}
                </option>
              ))}
            </select>
          </div>

          <div className={styles.field}>
            <div className={styles.toggleRow}>
              <label className={styles.label} htmlFor="pref-wm-opacity">
                {t('settings.opacity')}
              </label>
              <span className={styles.mono}>{wmOpacity.toFixed(2)}</span>
            </div>
            <input
              id="pref-wm-opacity"
              className={styles.slider}
              type="range"
              min={0.1}
              max={1}
              step={0.05}
              value={wmOpacity}
              onChange={(e) => setWmOpacity(Number(e.target.value))}
            />
          </div>

          <div className={styles.field}>
            <label className={styles.label} htmlFor="pref-wm-margin">
              {t('settings.margin')}
            </label>
            <input
              id="pref-wm-margin"
              className={styles.input}
              type="number"
              min={0}
              max={256}
              value={wmMargin}
              onChange={(e) => setWmMargin(Number(e.target.value) || 0)}
            />
          </div>

          <Button
            variant="primary"
            className={styles.saveBtn}
            disabled={save.isPending || policies.isPending}
            onClick={submit}
          >
            {t('settings.savePreferences')}
          </Button>
        </div>
      </div>
    </div>
  )
}
