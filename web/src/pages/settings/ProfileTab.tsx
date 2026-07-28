import { useQueryClient } from '@tanstack/react-query'
import { useRef, useState } from 'react'
import { useNavigate } from 'react-router'
import { ApiError } from '../../api/client'
import {
  useChangePassword,
  useDeleteAccount,
  useDeleteAvatar,
  useResendVerification,
  useSession,
  useChangeEmail,
  useUpdateProfile,
  useUploadAvatar,
} from '../../api/hooks'
import { useT } from '../../i18n'
import { errorText } from '../../i18n/errorText'
import { useGlobal } from '../../store'
import { Button } from '../../ui/Button'
import { InlineConfirm } from '../../ui/InlineConfirm'
import { Tag } from '../../ui/Tag'
import { Toggle } from '../../ui/Toggle'
import styles from './SettingsPage.module.css'

const STRONG_RE = /^(?=.*[A-Za-z])(?=.*\d).{8,}$/

export function ProfileTab() {
  const { t } = useT()
  const { data: user } = useSession()
  const updateProfile = useUpdateProfile()
  const changePwd = useChangePassword()
  const changeEmail = useChangeEmail()
  const resend = useResendVerification()
  const uploadAvatar = useUploadAvatar()
  const deleteAvatar = useDeleteAvatar()
  const deleteAccount = useDeleteAccount()
  const pushToast = useGlobal((s) => s.pushToast)
  const qc = useQueryClient()
  const navigate = useNavigate()
  const fileRef = useRef<HTMLInputElement>(null)
  const [nick, setNick] = useState<string | null>(null)
  const [oldPwd, setOldPwd] = useState('')
  const [newPwd, setNewPwd] = useState('')
  const [pwdErr, setPwdErr] = useState<string | null>(null)
  const [delPwd, setDelPwd] = useState('')
  const [delErr, setDelErr] = useState<string | null>(null)
  const [newEmail, setNewEmail] = useState('')
  const [emailPwd, setEmailPwd] = useState('')
  const [emailErr, setEmailErr] = useState<string | null>(null)

  if (!user) return null
  const nickVal = nick ?? user.nickname ?? ''

  function savePwd() {
    setPwdErr(null)
    if (!oldPwd) return setPwdErr(t('settings.errCurrentPasswordRequired'))
    if (!STRONG_RE.test(newPwd)) return setPwdErr(t('settings.errPasswordWeak'))
    changePwd.mutate(
      { old_password: oldPwd, new_password: newPwd },
      {
        onSuccess: () => {
          setOldPwd('')
          setNewPwd('')
          pushToast(t('settings.toastPasswordUpdated'))
        },
        onError: (e) =>
          setPwdErr(
            e instanceof ApiError && e.code === 'invalid_credentials'
              ? t('settings.errCurrentPasswordWrong')
              : errorText((e as ApiError).code, (e as Error).message),
          ),
      },
    )
  }

  return (
    <div>
      <div className={styles.section}>
        <div className={styles.kicker}>{t('settings.profileKicker')}</div>
        <div className={styles.card}>
          <div className={styles.avatarRow}>
            {user.avatar_url ? (
              <img className={styles.avatarPreview} src={user.avatar_url} alt={t('settings.avatarAlt')} />
            ) : (
              <div className={styles.avatarFallback}>{(user.nickname || user.username).slice(0, 1)}</div>
            )}
            <div className={styles.avatarActions}>
              <input
                ref={fileRef}
                type="file"
                accept="image/*"
                hidden
                onChange={(e) => {
                  const f = e.target.files?.[0]
                  if (f) uploadAvatar.mutate(f, { onSuccess: () => pushToast(t('settings.toastAvatarUpdated')) })
                  e.target.value = ''
                }}
              />
              <Button variant="secondary" disabled={uploadAvatar.isPending} onClick={() => fileRef.current?.click()}>
                {t('settings.uploadAvatar')}
              </Button>
              {user.avatar_url && (
                <InlineConfirm
                  label={t('settings.removeAvatar')}
                  confirmLabel={t('settings.confirmRemoveAvatar')}
                  onConfirm={() => deleteAvatar.mutate()}
                />
              )}
            </div>
          </div>
          <div className={styles.grid2}>
            <div className={styles.field}>
              <label className={styles.label} htmlFor="nick">{t('settings.nickname')}</label>
              <input id="nick" className={styles.input} value={nickVal} onChange={(e) => setNick(e.target.value)} />
            </div>
            <div className={styles.field}>
              <label className={styles.label} htmlFor="email">{t('settings.email')}</label>
              <input id="email" className={`${styles.input} ${styles.inputRO}`} value={user.email} readOnly />
              <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
                {user.email_verified ? (
                  <Tag variant="ok">{t('settings.verified')}</Tag>
                ) : (
                  <>
                    <Tag variant="warn">{t('settings.unverified')}</Tag>
                    <Button
                      variant="secondary"
                      disabled={resend.isPending}
                      onClick={() =>
                        resend.mutate(undefined, {
                          onSuccess: () => pushToast(t('settings.toastVerificationSent')),
                        })
                      }
                    >
                      {t('settings.resendVerification')}
                    </Button>
                  </>
                )}
              </div>
              <div className={styles.field} style={{ marginTop: 12 }}>
                <label className={styles.label} htmlFor="new-email">
                  {t('settings.changeEmail')}
                </label>
                <input
                  id="new-email"
                  className={styles.input}
                  value={newEmail}
                  onChange={(e) => setNewEmail(e.target.value)}
                  placeholder={t('settings.newEmailPlaceholder')}
                />
                <input
                  type="password"
                  className={styles.input}
                  style={{ marginTop: 8 }}
                  value={emailPwd}
                  onChange={(e) => setEmailPwd(e.target.value)}
                  placeholder={t('settings.confirmPassword')}
                />
                {emailErr && <div className={styles.err}>{emailErr}</div>}
                <Button
                  variant="secondary"
                  style={{ marginTop: 8 }}
                  disabled={changeEmail.isPending}
                  onClick={() => {
                    setEmailErr(null)
                    changeEmail.mutate(
                      { password: emailPwd, new_email: newEmail.trim() },
                      {
                        onSuccess: () => {
                          setNewEmail('')
                          setEmailPwd('')
                          pushToast(t('settings.toastChangeEmailSent'))
                        },
                        onError: (e) =>
                          setEmailErr(
                            e instanceof ApiError ? errorText(e.code, e.message) : t('settings.errGeneric'),
                          ),
                      },
                    )
                  }}
                >
                  {t('settings.sendChangeEmail')}
                </Button>
              </div>
            </div>
          </div>
          <div className={styles.field}>
            <div className={styles.toggleRow}>
              <span className={styles.label}>{t('settings.publicProfile')}</span>
              <Toggle
                checked={!!user.public_profile}
                onChange={(v) =>
                  updateProfile.mutate({ public_profile: v }, { onSuccess: () => pushToast(t('settings.toastSaved')) })
                }
                aria-label={t('settings.publicProfileAria')}
              />
            </div>
            <div className={styles.label} style={{ fontWeight: 400 }}>
              {t('settings.publicProfileHint', { username: user.username })}
            </div>
          </div>
          <Button
            variant="primary"
            className={styles.saveBtn}
            disabled={updateProfile.isPending}
            onClick={() =>
              updateProfile.mutate({ nickname: nickVal.trim() }, { onSuccess: () => pushToast(t('settings.toastSaved')) })
            }
          >
            {t('settings.saveChanges')}
          </Button>
        </div>
      </div>

      <div className={styles.section}>
        <div className={styles.kicker}>{t('settings.passwordKicker')}</div>
        <div className={styles.card}>
          <div className={styles.grid2}>
            <div className={styles.field}>
              <label className={styles.label} htmlFor="old-pwd">{t('settings.currentPassword')}</label>
              <input id="old-pwd" type="password" className={styles.input} value={oldPwd} onChange={(e) => setOldPwd(e.target.value)} />
            </div>
            <div className={styles.field}>
              <label className={styles.label} htmlFor="new-pwd">{t('settings.newPassword')}</label>
              <input
                id="new-pwd"
                type="password"
                placeholder={t('settings.newPasswordPlaceholder')}
                className={styles.input}
                value={newPwd}
                onChange={(e) => setNewPwd(e.target.value)}
              />
            </div>
          </div>
          {pwdErr && <div className={styles.errLine}>{pwdErr}</div>}
          <Button className={styles.saveBtn} disabled={changePwd.isPending} onClick={savePwd}>
            {t('settings.updatePassword')}
          </Button>
        </div>
      </div>

      <div className={styles.section}>
        <div className={styles.kicker}>{t('settings.dangerKicker')}</div>
        <div className={`${styles.card} ${styles.dangerCard}`}>
          <p className={styles.dangerText}>
            {t('settings.dangerTextBefore')}
            <strong>{t('settings.dangerTextStrong')}</strong>
            {t('settings.dangerTextAfter')}
          </p>
          <div className={styles.field}>
            <label className={styles.label} htmlFor="del-pwd">{t('settings.deleteConfirmPassword')}</label>
            <input
              id="del-pwd"
              type="password"
              className={styles.input}
              value={delPwd}
              onChange={(e) => setDelPwd(e.target.value)}
            />
          </div>
          {delErr && <div className={styles.errLine}>{delErr}</div>}
          <InlineConfirm
            label={t('settings.deleteAccount')}
            confirmLabel={t('settings.confirmDeleteAccount')}
            disabled={!delPwd || deleteAccount.isPending}
            onConfirm={() => {
              setDelErr(null)
              deleteAccount.mutate(
                { password: delPwd },
                {
                  onSuccess: () => {
                    pushToast(t('settings.toastAccountDeleted'))
                    qc.clear()
                    navigate('/login')
                  },
                  onError: (e) =>
                    setDelErr(
                      e instanceof ApiError && e.code === 'invalid_credentials'
                        ? t('settings.errPasswordWrong')
                        : e instanceof ApiError && e.code === 'admin_cannot_self_delete'
                          ? t('settings.errAdminCannotSelfDelete')
                          : errorText((e as ApiError).code, (e as Error).message),
                    ),
                },
              )
            }}
          />
        </div>
      </div>
    </div>
  )
}
