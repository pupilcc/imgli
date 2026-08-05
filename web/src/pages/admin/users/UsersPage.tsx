import { useEffect, useState } from 'react'
import { useAdminGroups, useAdminUsers, useResetAdminPassword, useUpdateAdminUser } from '../../../api/adminHooks'
import { useSession } from '../../../api/hooks'
import type { AdminUser } from '../../../api/types'
import { useT } from '../../../i18n'
import { cn } from '../../../lib/cn'
import { copyText } from '../../../lib/copy'
import { formatBytes, formatDate } from '../../../lib/format'
import { useAdminSearchParam } from '../../../lib/useAdminSearchParam'
import { useDebounced } from '../../../lib/useDebounced'
import { PageHeader } from '../../../shell/PageHeader'
import { ArmedButton } from '../../../ui/ArmedButton'
import { Button } from '../../../ui/Button'
import { EmptyState } from '../../../ui/EmptyState'
import { Modal } from '../../../ui/Modal'
import {
  AdminFilters,
  AdminSearch,
  AdminSelect,
  AdminSortTh,
  AdminTable,
  AdminTableHead,
  AdminTableRow,
  IconLink,
  StatusPill,
  iconActionArmedClass,
  iconActionArmedOkClass,
  iconActionClass,
} from '../ui/adminChrome'
import { AdminQueryGate } from '../ui/AdminQueryGate'
import { Pager } from '../ui/Pager'

type SortKey = '' | 'bandwidth' | 'storage' | 'created' | 'last_seen'

/** 主表：用户 · 组 · 用量(叠) · 状态 · 操作；次要字段下沉到展开行 */
const COLS = 'minmax(168px,1.5fr) minmax(100px,0.85fr) minmax(140px,1.15fr) minmax(56px,0.45fr) minmax(104px,auto)'

function sortLabel(t: (k: string) => string, sort: string): string {
  switch (sort) {
    case 'bandwidth':
      return t('adminA.sortBandwidth')
    case 'storage':
      return t('adminA.sortStorage')
    case 'created':
      return t('adminA.sortCreated')
    case 'last_seen':
      return t('adminA.sortLastSeen')
    default:
      return t('adminA.sortId')
  }
}

function UsageStack({
  storageUsed,
  storageQuota,
  bwUsed,
  bwQuota,
  bwPeriod,
  storageLabel,
  bandwidthLabel,
}: {
  storageUsed: number
  storageQuota: number
  bwUsed: number
  bwQuota: number
  bwPeriod?: string
  storageLabel: string
  bandwidthLabel: string
}) {
  const storagePct = storageQuota > 0 ? Math.min(100, Math.round((storageUsed / storageQuota) * 100)) : 0
  const bwPct = bwQuota > 0 ? Math.min(100, Math.round((bwUsed / bwQuota) * 100)) : 0
  return (
    <div className="flex w-full min-w-0 flex-col gap-1.5 justify-self-end text-right">
      <div className="flex min-w-0 items-center justify-end gap-2" title={storageLabel}>
        {storageQuota > 0 && (
          <span className="inline-block h-[3px] w-8 flex-none overflow-hidden rounded-sm bg-soft" title={`${storagePct}%`}>
            <span className="block h-full bg-ink" style={{ width: `${storagePct}%` }} />
          </span>
        )}
        <span className="whitespace-nowrap font-mono text-xs-plus text-muted tabular-nums">
          <span className="mr-1 text-[10px] tracking-[0.04em] text-muted opacity-80">{storageLabel}</span>
          {formatBytes(storageUsed)}
        </span>
      </div>
      <div className="flex min-w-0 items-center justify-end gap-2" title={bandwidthLabel}>
        {bwQuota > 0 && (
          <span className="inline-block h-[3px] w-8 flex-none overflow-hidden rounded-sm bg-soft" title={`${bwPct}%`}>
            <span className="block h-full bg-ink" style={{ width: `${bwPct}%` }} />
          </span>
        )}
        <span className="whitespace-nowrap font-mono text-xs-plus text-muted tabular-nums">
          <span className="mr-1 text-[10px] tracking-[0.04em] text-muted opacity-80">{bandwidthLabel}</span>
          {formatBytes(bwUsed)}
          {bwPeriod ? ` · ${bwPeriod}` : ''}
        </span>
      </div>
    </div>
  )
}

export function UsersPage() {
  const { t } = useT()
  const { params, setParams, setParam } = useAdminSearchParam()
  const q = params.get('q') ?? ''
  const group = Number(params.get('group')) || undefined
  const status = params.get('status') ?? ''
  const page = Number(params.get('page')) || 1

  const [input, setInput] = useState(q)
  useEffect(() => {
    setInput(q)
  }, [q])
  const debouncedInput = useDebounced(input, 300)
  useEffect(() => {
    setParams((p) => {
      if ((p.get('q') ?? '') === debouncedInput) return p
      const n = new URLSearchParams(p)
      if (debouncedInput) n.set('q', debouncedInput)
      else n.delete('q')
      n.delete('page')
      return n
    })
  }, [debouncedInput])

  const { data: me } = useSession()
  const channel = params.get('channel') ?? ''
  const sort = (params.get('sort') ?? '') as SortKey
  const users = useAdminUsers({
    q: q || undefined,
    group,
    status: status || undefined,
    channel: channel || undefined,
    sort: sort || undefined,
    page,
  })
  const groupsQ = useAdminGroups()
  const update = useUpdateAdminUser()
  const reset = useResetAdminPassword()
  const [resetTarget, setResetTarget] = useState<AdminUser | null>(null)
  const [expanded, setExpanded] = useState<Set<number>>(() => new Set())

  const groups = groupsQ.data?.items ?? []
  const groupName = (id: number) => groups.find((g) => g.id === id)?.name ?? `#${id}`
  const groupQuota = (id: number) => groups.find((g) => g.id === id)?.storage_quota ?? 0
  const groupBwQuota = (id: number) => groups.find((g) => g.id === id)?.bandwidth_quota_month ?? 0

  const closeReset = () => {
    setResetTarget(null)
    reset.reset()
  }

  const setSort = (key: SortKey) => {
    setParams((p) => {
      const n = new URLSearchParams(p)
      const cur = n.get('sort') ?? ''
      if (cur === key) n.delete('sort')
      else if (key) n.set('sort', key)
      else n.delete('sort')
      n.delete('page')
      return n
    })
  }

  const toggleExpand = (id: number) => {
    setExpanded((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  return (
    <div>
      <PageHeader
        kicker="USERS"
        title={t('adminA.usersTitle')}
        extra={
          <AdminFilters>
            <AdminSearch value={input} onChange={setInput} placeholder={t('adminA.searchUsersPlaceholder')} />
            <AdminSelect value={group ?? ''} onChange={(e) => setParam('group', e.target.value)} aria-label={t('adminA.filterGroupAria')}>
              <option value="">{t('adminA.allGroups')}</option>
              {groups.map((g) => (
                <option key={g.id} value={g.id}>
                  {g.name}
                </option>
              ))}
            </AdminSelect>
            <AdminSelect value={status} onChange={(e) => setParam('status', e.target.value)} aria-label={t('adminA.filterStatusAria')}>
              <option value="">{t('adminA.allStatuses')}</option>
              <option value="active">{t('adminA.activeUsers')}</option>
              <option value="banned">{t('adminA.bannedUsers')}</option>
            </AdminSelect>
            <AdminSelect value={channel} onChange={(e) => setParam('channel', e.target.value)} aria-label={t('adminA.filterChannelAria')}>
              <option value="">{t('adminA.allChannels')}</option>
              <option value="direct">{t('adminA.channelDirect')}</option>
              <option value="invite">{t('adminA.channelInvite')}</option>
              <option value="utm">{t('adminA.channelUtm')}</option>
              <option value="referer">{t('adminA.channelReferer')}</option>
            </AdminSelect>
            <AdminSelect
              value={sort}
              onChange={(e) => setSort((e.target.value || '') as SortKey)}
              aria-label={t('adminA.sortBy')}
            >
              <option value="">{t('adminA.sortId')}</option>
              <option value="storage">{t('adminA.sortStorage')}</option>
              <option value="bandwidth">{t('adminA.sortBandwidth')}</option>
              <option value="created">{t('adminA.sortCreated')}</option>
              <option value="last_seen">{t('adminA.sortLastSeen')}</option>
            </AdminSelect>
            <a
              className="inline-flex h-[34px] items-center rounded-sm border border-border bg-surface px-3 font-mono text-xs text-ink no-underline hover:border-muted"
              href={`/api/v1/admin/export/users.csv?${new URLSearchParams({
                ...(q ? { q } : {}),
                ...(group ? { group: String(group) } : {}),
                ...(status ? { status } : {}),
                ...(channel ? { channel } : {}),
                ...(sort ? { sort } : {}),
              }).toString()}`}
            >
              CSV
            </a>
          </AdminFilters>
        }
      />
      <AdminQueryGate query={users}>
        {(data) =>
          data.items.length === 0 ? (
            <EmptyState title={t('adminA.noMatchingUsers')} desc={t('adminA.noMatchingUsersDesc')} />
          ) : (
            <>
              <AdminTable minWidth={720}>
                <AdminTableHead columns={COLS}>
                  <span className="justify-self-start">{t('adminA.colUser')}</span>
                  <span className="justify-self-start">{t('adminA.colGroup')}</span>
                  <div className="flex w-full flex-col items-end gap-0.5 justify-self-end">
                    <AdminSortTh
                      label={t('adminA.usageStorageLabel')}
                      sortAria={t('adminA.sortAria', { col: t('adminA.colUsedStorage') })}
                      active={sort === 'storage'}
                      align="end"
                      onClick={() => setSort('storage')}
                    />
                    <AdminSortTh
                      label={t('adminA.usageBandwidthLabel')}
                      sortAria={t('adminA.sortAria', { col: t('adminA.colBandwidth') })}
                      active={sort === 'bandwidth'}
                      align="end"
                      onClick={() => setSort('bandwidth')}
                    />
                  </div>
                  <span className="justify-self-start">{t('adminA.colStatus')}</span>
                  <span className="justify-self-end">{t('adminA.colActions')}</span>
                </AdminTableHead>
                {data.items.map((u) => {
                  const usedBw = u.bandwidth_used_month ?? 0
                  const open = expanded.has(u.id)
                  return (
                    <div key={u.id}>
                      <AdminTableRow columns={COLS}>
                        <div className="flex min-w-0 w-full items-center gap-2 justify-self-start">
                          <button
                            type="button"
                            className={cn(
                              'flex h-6 w-6 flex-none cursor-pointer items-center justify-center rounded-sm border border-border bg-surface font-mono text-[11px] text-muted hover:border-muted hover:text-ink',
                              open && 'border-ink text-ink',
                            )}
                            aria-expanded={open}
                            aria-label={open ? t('adminA.collapseUserDetails') : t('adminA.expandUserDetails')}
                            onClick={() => toggleExpand(u.id)}
                          >
                            {open ? '▾' : '▸'}
                          </button>
                          <span className="flex h-[26px] w-[26px] flex-none items-center justify-center rounded-full border border-border bg-soft text-[11px] font-bold">
                            {(u.nickname || u.username).slice(0, 1)}
                          </span>
                          <div className="flex min-w-0 flex-col gap-0.5">
                            <span className="overflow-hidden text-ellipsis whitespace-nowrap text-[13px] font-bold">{u.username}</span>
                            <span
                              className="overflow-hidden text-ellipsis whitespace-nowrap font-mono text-xs-plus text-muted"
                              title={u.email}
                            >
                              {u.email}
                              {u.email_verified === false && (
                                <span className="text-err opacity-85"> · {t('adminA.emailVerifiedNo')}</span>
                              )}
                            </span>
                          </div>
                        </div>
                        <select
                          className="h-[26px] w-full max-w-full justify-self-start rounded-sm border border-border bg-surface px-1.5 font-mono text-xs-plus text-ink"
                          value={u.group_id}
                          aria-label={t('adminA.userGroupAria', { username: u.username })}
                          onChange={(e) => update.mutate({ id: u.id, body: { group_id: Number(e.target.value) } })}
                        >
                          {groups.map((g) => (
                            <option key={g.id} value={g.id}>
                              {g.name}
                            </option>
                          ))}
                          {groups.length === 0 && <option value={u.group_id}>{groupName(u.group_id)}</option>}
                        </select>
                        <UsageStack
                          storageUsed={u.used_storage}
                          storageQuota={groupQuota(u.group_id)}
                          bwUsed={usedBw}
                          bwQuota={groupBwQuota(u.group_id)}
                          bwPeriod={u.bandwidth_period}
                          storageLabel={t('adminA.usageStorageLabel')}
                          bandwidthLabel={t('adminA.usageBandwidthLabel')}
                        />
                        <StatusPill ok={u.status === 'active'}>
                          {u.status === 'active' ? t('adminA.statusActive') : t('adminA.statusBanned')}
                        </StatusPill>
                        <div className="flex w-full items-center justify-end gap-1 justify-self-end">
                          {u.status === 'active' ? (
                            <ArmedButton
                              className={iconActionClass}
                              armedClassName={iconActionArmedClass}
                              title={t('adminA.ban')}
                              armedTitle={t('adminA.confirmBan')}
                              armedChildren={t('adminA.confirmBan')}
                              disabled={me?.id === u.id}
                              onConfirm={() => update.mutate({ id: u.id, body: { status: 'banned' } })}
                            >
                              <span aria-hidden>⊘</span>
                            </ArmedButton>
                          ) : (
                            <ArmedButton
                              className={iconActionClass}
                              armedClassName={iconActionArmedOkClass}
                              title={t('adminA.unban')}
                              armedTitle={t('adminA.confirmUnban')}
                              armedChildren={t('adminA.confirmUnban')}
                              onConfirm={() => update.mutate({ id: u.id, body: { status: 'active' } })}
                            >
                              <span aria-hidden>○</span>
                            </ArmedButton>
                          )}
                          <ArmedButton
                            className={iconActionClass}
                            armedClassName={iconActionArmedClass}
                            title={t('adminA.resetPassword')}
                            armedTitle={t('adminA.confirmResetIcon')}
                            armedChildren={t('adminA.confirmResetIcon')}
                            onConfirm={() => setResetTarget(u)}
                          >
                            <span aria-hidden>⌁</span>
                          </ArmedButton>
                          <IconLink to={`/admin/images?user=${u.id}`} title={t('adminA.viewImages')}>
                            <span aria-hidden>▦</span>
                          </IconLink>
                        </div>
                      </AdminTableRow>
                      {open && (
                        <div className="grid grid-cols-1 gap-2 border-b border-border bg-soft/60 px-4 py-2.5 sm:grid-cols-3">
                          <div className="flex min-w-0 flex-col gap-0.5">
                            <span className="font-mono text-2xs tracking-[0.08em] text-muted uppercase">
                              {t('adminA.colImageCount')}
                            </span>
                            <span className="font-mono text-xs-plus tabular-nums text-ink">{u.image_count}</span>
                          </div>
                          <div className="flex min-w-0 flex-col gap-0.5">
                            <span className="font-mono text-2xs tracking-[0.08em] text-muted uppercase">
                              {t('adminA.colRegistered')}
                            </span>
                            <span className="font-mono text-xs-plus tabular-nums text-ink">{formatDate(u.created_at)}</span>
                          </div>
                          <div className="flex min-w-0 flex-col gap-0.5">
                            <span className="font-mono text-2xs tracking-[0.08em] text-muted uppercase">
                              {t('adminA.colLastSeen')}
                            </span>
                            <span className="font-mono text-xs-plus tabular-nums text-ink">
                              {u.last_seen_at ? formatDate(u.last_seen_at) : t('adminA.neverSeen')}
                            </span>
                          </div>
                        </div>
                      )}
                    </div>
                  )
                })}
              </AdminTable>
              <div className="mt-2.5 flex flex-wrap items-center justify-between gap-2.5">
                <p className="m-0 mx-0.5 font-mono text-xs-plus text-muted">
                  {t('adminA.usersTotal', { total: data.total })}
                  <span className="mx-1.5 opacity-50">·</span>
                  {t('adminA.usersPageHint', { page: data.page, limit: data.limit })}
                  <span className="mx-1.5 opacity-50">·</span>
                  {t('adminA.sortBy')}: {sortLabel(t, sort)}
                </p>
                <Pager page={page} limit={data.limit} total={data.total} onPage={(p) => setParam('page', p > 1 ? String(p) : '')} />
              </div>
            </>
          )
        }
      </AdminQueryGate>
      <Modal open={resetTarget !== null} onClose={closeReset} width={400}>
        {resetTarget && (
          <div className="flex flex-col gap-3">
            {reset.data ? (
              <>
                <h2 className="m-0 text-base font-bold">{t('adminA.passwordGenerated')}</h2>
                <div className="break-all rounded-sm border border-border bg-soft px-3 py-2.5 font-mono text-[15px] tracking-[0.04em]">
                  {reset.data.password}
                </div>
                <p className="m-0 text-sm-plus text-muted">{t('adminA.passwordOnceHint')}</p>
                <div className="flex justify-end gap-2">
                  <Button variant="primary" onClick={() => copyText(reset.data!.password, t('adminA.passwordLabel'))}>
                    {t('adminA.copyPassword')}
                  </Button>
                  <Button variant="secondary" onClick={closeReset}>
                    {t('common.close')}
                  </Button>
                </div>
              </>
            ) : (
              <>
                <h2 className="m-0 text-base font-bold">{t('adminA.resetPasswordTitle')}</h2>
                <p className="m-0 text-sm-plus text-muted">
                  {t('adminA.resetPasswordHintBefore')}
                  <b>{resetTarget.username}</b>
                  {t('adminA.resetPasswordHintAfter')}
                </p>
                <div className="flex justify-end gap-2">
                  <Button variant="primary" disabled={reset.isPending} onClick={() => reset.mutate(resetTarget.id)}>
                    {t('adminA.confirmReset')}
                  </Button>
                  <Button variant="secondary" onClick={closeReset}>
                    {t('common.cancel')}
                  </Button>
                </div>
              </>
            )}
          </div>
        )}
      </Modal>
    </div>
  )
}
