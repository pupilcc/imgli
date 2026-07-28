import { useState } from 'react'
import { useSearchParams } from 'react-router'
import { useAdminInvites, useCreateInvites, useRevokeInvite } from '../../../api/adminHooks'
import type { AdminInvite } from '../../../api/types'
import { useT } from '../../../i18n'
import { copyText } from '../../../lib/copy'
import { formatDate } from '../../../lib/format'
import { PageHeader } from '../../../shell/PageHeader'
import { useGlobal } from '../../../store'
import { Button } from '../../../ui/Button'
import { EmptyState } from '../../../ui/EmptyState'
import { InlineConfirm } from '../../../ui/InlineConfirm'
import { Input } from '../../../ui/Input'
import { Modal } from '../../../ui/Modal'
import { Skeleton } from '../../../ui/Skeleton'
import { Tag } from '../../../ui/Tag'
import { AdminError } from '../ui/AdminError'
import { Pager } from '../ui/Pager'
import styles from './InvitesPage.module.css'

export function InvitesPage() {
  const { t } = useT()
  const [params, setParams] = useSearchParams()
  const status = params.get('status') ?? ''
  const page = Number(params.get('page')) || 1
  const [modalOpen, setModalOpen] = useState(false)
  const [count, setCount] = useState('10')
  const [days, setDays] = useState('')
  const [madeCodes, setMadeCodes] = useState<string[] | null>(null)

  const statusTag = (s: AdminInvite['status']): { label: string; variant: 'ok' | 'muted' | 'warn' } => {
    if (s === 'unused') return { label: t('adminB.statusUnused'), variant: 'ok' }
    if (s === 'used') return { label: t('adminB.statusUsed'), variant: 'muted' }
    return { label: t('adminB.statusExpired'), variant: 'warn' }
  }

  const setParam = (key: string, value: string) => {
    setParams((p) => {
      const n = new URLSearchParams(p)
      if (value) n.set(key, value)
      else n.delete(key)
      if (key !== 'page') n.delete('page')
      return n
    })
  }

  const invites = useAdminInvites({ status: status || undefined, page })
  const create = useCreateInvites()
  const revoke = useRevokeInvite()

  const submitCreate = () => {
    const n = Number(count)
    if (!Number.isInteger(n) || n < 1 || n > 100) {
      useGlobal.getState().pushToast(t('adminB.countRangeToast'))
      return
    }
    const d = Number(days) || 0
    create.mutate(
      { count: n, ...(d > 0 ? { expires_in_days: d } : {}) },
      { onSuccess: (data) => setMadeCodes(data.codes) },
    )
  }

  const closeModal = () => {
    setModalOpen(false)
    setMadeCodes(null)
  }

  return (
    <div>
      <PageHeader
        kicker="INVITE CODES"
        title={t('adminB.invitesTitle')}
        extra={
          <div className={styles.toolbar}>
            <select value={status} onChange={(e) => setParam('status', e.target.value)} aria-label={t('adminB.filterStatusAria')} className={styles.select}>
              <option value="">{t('adminB.allStatuses')}</option>
              <option value="unused">{t('adminB.statusUnused')}</option>
              <option value="used">{t('adminB.statusUsed')}</option>
              <option value="expired">{t('adminB.statusExpired')}</option>
            </select>
            <Button
              variant="primary"
              onClick={() => {
                setMadeCodes(null)
                setModalOpen(true)
              }}
            >
              {t('adminB.generateInvites')}
            </Button>
          </div>
        }
      />
      {invites.isError ? (
        <AdminError onRetry={() => invites.refetch()} />
      ) : !invites.data ? (
        <Skeleton height={220} />
      ) : invites.data.items.length === 0 ? (
        invites.data.total > 0 ? (
          <EmptyState badge="✓" title={t('adminB.pageCleared')} desc={t('adminB.pageClearedInvitesDesc')}>
            <Button variant="secondary" onClick={() => setParam('page', '')}>
              {t('adminB.backToPage1')}
            </Button>
          </EmptyState>
        ) : (
          <EmptyState title={t('adminB.noInvites')} desc={t('adminB.noInvitesDesc')} />
        )
      ) : (
        <>
          <div className={styles.table}>
            <div className={`${styles.head} ${styles.row}`}>
              <span>{t('adminB.colCode')}</span>
              <span>{t('adminB.colStatus')}</span>
              <span>{t('adminB.colUsedBy')}</span>
              <span>{t('adminB.colCreatedExpires')}</span>
              <span />
            </div>
            {invites.data.items.map((ic) => {
              const tag = statusTag(ic.status)
              return (
                <div key={ic.id} className={styles.row}>
                  <span className={styles.code}>{ic.code}</span>
                  <span>
                    <Tag variant={tag.variant}>{tag.label}</Tag>
                  </span>
                  <span className={styles.by}>{ic.used_by_name || '—'}</span>
                  <span className={styles.time}>
                    {formatDate(ic.created_at)}
                    {ic.expires_at ? ` / ${formatDate(ic.expires_at)}` : ` / ${t('adminB.permanent')}`}
                  </span>
                  <span className={styles.ops}>
                    {ic.status !== 'used' && (
                      <InlineConfirm label={t('adminB.revoke')} confirmLabel={t('adminB.confirmRevoke')} onConfirm={() => revoke.mutate(ic.id)} />
                    )}
                  </span>
                </div>
              )
            })}
          </div>
          <Pager page={page} limit={invites.data.limit} total={invites.data.total} onPage={(p) => setParam('page', p > 1 ? String(p) : '')} />
        </>
      )}

      <Modal open={modalOpen} onClose={closeModal}>
        {madeCodes ? (
          <div className={styles.modalBody}>
            <h2 className={styles.modalTitle}>{t('adminB.generatedCount', { count: madeCodes.length })}</h2>
            <pre className={styles.codeList}>{madeCodes.join('\n')}</pre>
            <div className={styles.modalActions}>
              <Button variant="secondary" onClick={() => copyText(madeCodes.join('\n'), t('adminB.inviteCodesLabel', { count: madeCodes.length }))}>
                {t('adminB.copyAll')}
              </Button>
              <Button variant="primary" onClick={closeModal}>
                {t('adminB.done')}
              </Button>
            </div>
          </div>
        ) : (
          <div className={styles.modalBody}>
            <h2 className={styles.modalTitle}>{t('adminB.generateInvites')}</h2>
            <Input label={t('adminB.countLabel')} type="number" value={count} onChange={(e) => setCount(e.target.value)} />
            <Input label={t('adminB.expiresDays')} type="number" placeholder={t('adminB.expiresPlaceholder')} value={days} onChange={(e) => setDays(e.target.value)} />
            <div className={styles.modalActions}>
              <Button variant="secondary" onClick={closeModal}>
                {t('common.cancel')}
              </Button>
              <Button variant="primary" disabled={create.isPending} onClick={submitCreate}>
                {t('adminB.generate')}
              </Button>
            </div>
          </div>
        )}
      </Modal>
    </div>
  )
}
