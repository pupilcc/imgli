import { useState } from 'react'
import { useAdminInvites, useCreateInvites, useRevokeInvite } from '../../../api/adminHooks'
import type { AdminInvite } from '../../../api/types'
import { useT } from '../../../i18n'
import { copyText } from '../../../lib/copy'
import { formatDate } from '../../../lib/format'
import { useAdminSearchParam } from '../../../lib/useAdminSearchParam'
import { PageHeader } from '../../../shell/PageHeader'
import { useGlobal } from '../../../store'
import { Button } from '../../../ui/Button'
import { EmptyState } from '../../../ui/EmptyState'
import { InlineConfirm } from '../../../ui/InlineConfirm'
import { Input } from '../../../ui/Input'
import { Modal } from '../../../ui/Modal'
import { Tag } from '../../../ui/Tag'
import {
  AdminFilters,
  AdminSelect,
  AdminTable,
  AdminTableHead,
  AdminTableRow,
} from '../ui/adminChrome'
import { AdminQueryGate } from '../ui/AdminQueryGate'
import { Pager } from '../ui/Pager'

const COLS = '150px 90px 1fr minmax(180px,220px) 90px'

export function InvitesPage() {
  const { t } = useT()
  const { params, setParam } = useAdminSearchParam()
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
          <AdminFilters>
            <AdminSelect
              value={status}
              onChange={(e) => setParam('status', e.target.value)}
              aria-label={t('adminB.filterStatusAria')}
            >
              <option value="">{t('adminB.allStatuses')}</option>
              <option value="unused">{t('adminB.statusUnused')}</option>
              <option value="used">{t('adminB.statusUsed')}</option>
              <option value="expired">{t('adminB.statusExpired')}</option>
            </AdminSelect>
            <Button
              variant="primary"
              onClick={() => {
                setMadeCodes(null)
                setModalOpen(true)
              }}
            >
              {t('adminB.generateInvites')}
            </Button>
          </AdminFilters>
        }
      />
      <AdminQueryGate query={invites}>
        {(data) =>
          data.items.length === 0 ? (
            data.total > 0 ? (
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
              <AdminTable minWidth={640}>
                <AdminTableHead columns={COLS} className="text-[11px] tracking-[0.04em] normal-case">
                  <span>{t('adminB.colCode')}</span>
                  <span>{t('adminB.colStatus')}</span>
                  <span>{t('adminB.colUsedBy')}</span>
                  <span>{t('adminB.colCreatedExpires')}</span>
                  <span />
                </AdminTableHead>
                {data.items.map((ic) => {
                  const tag = statusTag(ic.status)
                  return (
                    <AdminTableRow key={ic.id} columns={COLS} className="text-[13px]">
                      <span className="font-mono font-semibold">{ic.code}</span>
                      <span>
                        <Tag variant={tag.variant}>{tag.label}</Tag>
                      </span>
                      <span className="text-muted">{ic.used_by_name || '—'}</span>
                      <span className="text-xs text-muted">
                        {formatDate(ic.created_at)}
                        {ic.expires_at ? ` / ${formatDate(ic.expires_at)}` : ` / ${t('adminB.permanent')}`}
                      </span>
                      <span className="justify-self-end text-right">
                        {ic.status !== 'used' && (
                          <InlineConfirm
                            label={t('adminB.revoke')}
                            confirmLabel={t('adminB.confirmRevoke')}
                            onConfirm={() => revoke.mutate(ic.id)}
                          />
                        )}
                      </span>
                    </AdminTableRow>
                  )
                })}
              </AdminTable>
              <Pager page={page} limit={data.limit} total={data.total} onPage={(p) => setParam('page', p > 1 ? String(p) : '')} />
            </>
          )
        }
      </AdminQueryGate>

      <Modal open={modalOpen} onClose={closeModal}>
        {madeCodes ? (
          <div className="flex flex-col gap-3.5">
            <h2 className="m-0 text-[15px] font-bold">{t('adminB.generatedCount', { count: madeCodes.length })}</h2>
            <pre className="m-0 max-h-[260px] overflow-auto border border-border bg-soft p-3 font-mono text-[13px] leading-loose">
              {madeCodes.join('\n')}
            </pre>
            <div className="flex justify-end gap-2.5">
              <Button
                variant="secondary"
                onClick={() => copyText(madeCodes.join('\n'), t('adminB.inviteCodesLabel', { count: madeCodes.length }))}
              >
                {t('adminB.copyAll')}
              </Button>
              <Button variant="primary" onClick={closeModal}>
                {t('adminB.done')}
              </Button>
            </div>
          </div>
        ) : (
          <div className="flex flex-col gap-3.5">
            <h2 className="m-0 text-[15px] font-bold">{t('adminB.generateInvites')}</h2>
            <Input label={t('adminB.countLabel')} type="number" value={count} onChange={(e) => setCount(e.target.value)} />
            <Input
              label={t('adminB.expiresDays')}
              type="number"
              placeholder={t('adminB.expiresPlaceholder')}
              value={days}
              onChange={(e) => setDays(e.target.value)}
            />
            <div className="flex justify-end gap-2.5">
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
