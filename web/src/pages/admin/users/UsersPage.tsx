import { useEffect, useState } from 'react'
import { Link } from 'react-router'
import { useAdminGroups, useAdminUsers, useResetAdminPassword, useUpdateAdminUser } from '../../../api/adminHooks'
import { useSession } from '../../../api/hooks'
import type { AdminUser } from '../../../api/types'
import { useT } from '../../../i18n'
import { copyText } from '../../../lib/copy'
import { formatBytes, formatDate } from '../../../lib/format'
import { useAdminSearchParam } from '../../../lib/useAdminSearchParam'
import { useDebounced } from '../../../lib/useDebounced'
import { PageHeader } from '../../../shell/PageHeader'
import { ArmedButton } from '../../../ui/ArmedButton'
import { Button } from '../../../ui/Button'
import { EmptyState } from '../../../ui/EmptyState'
import { Modal } from '../../../ui/Modal'
import { AdminQueryGate } from '../ui/AdminQueryGate'
import { Pager } from '../ui/Pager'
import forms from '../ui/adminForms.module.css'
import styles from './UsersPage.module.css'

type SortKey = '' | 'bandwidth' | 'storage' | 'created' | 'last_seen'

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

export function UsersPage() {
  const { t } = useT()
  const { params, setParams, setParam } = useAdminSearchParam()
  const q = params.get('q') ?? ''
  const group = Number(params.get('group')) || undefined
  const status = params.get('status') ?? ''
  const page = Number(params.get('page')) || 1

  const [input, setInput] = useState(q)
  // URL 是筛选唯一真源:q 因非输入原因变化(后退/外部跳转再返回)时把 input 同步回 q。
  useEffect(() => {
    setInput(q)
  }, [q])
  const debouncedInput = useDebounced(input, 300)
  // 仅在 debounced 值变化时推 URL;用函数式更新读当前 q,值相等则原样返回不导航——
  // 外部改 q(setInput 同步)不会触发本 effect,故不会把旧输入覆盖回去。
  //
  // 注意:setParams(即 useSearchParams 的 setter)在本项目所用 react-router v7 中
  // 并非引用稳定——它是 useCallback([navigate, searchParams]),q 变化就会换身份。
  // 若把它放进本 effect 的依赖数组,任何外部改 q(包括上面的反向同步)都会误触发本
  // effect,用尚未追上的旧 debouncedInput 把刚同步回去的 q 覆盖掉。因此故意不依赖
  // setParams:本 effect 只应在 debouncedInput 变化时触发,调用的仍是本次渲染闭包
  // 里最新的 setParams,足够新鲜。
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
      // 再点同一列 → 回默认 id
      if (cur === key || (!key && !cur)) {
        if (!key) return p
        n.delete('sort')
      } else if (key) {
        n.set('sort', key)
      } else {
        n.delete('sort')
      }
      n.delete('page')
      return n
    })
  }

  const SortTh = ({
    col,
    label,
    align = 'start',
  }: {
    col: SortKey
    label: string
    align?: 'start' | 'end' | 'center'
  }) => {
    const active = (sort || '') === col
    return (
      <button
        type="button"
        className={[styles.thBtn, styles[`align${align[0].toUpperCase()}${align.slice(1)}`], active && styles.thActive]
          .filter(Boolean)
          .join(' ')}
        aria-label={t('adminA.sortAria', { col: label })}
        aria-pressed={active}
        title={active ? t('adminA.sortActive') : t('adminA.sortAria', { col: label })}
        onClick={() => setSort(col)}
      >
        <span>{label}</span>
        <span className={styles.thMark} aria-hidden>
          {active ? '▼' : '↕'}
        </span>
      </button>
    )
  }

  return (
    <div>
      <PageHeader
        kicker="USERS"
        title={t('adminA.usersTitle')}
        extra={
          <div className={forms.filters}>
            <div className={styles.search}>
              <span className={styles.searchGlyph}>⌕</span>
              <input
                value={input}
                onChange={(e) => setInput(e.target.value)}
                placeholder={t('adminA.searchUsersPlaceholder')}
              />
            </div>
            <select
              value={group ?? ''}
              onChange={(e) => setParam('group', e.target.value)}
              className={forms.select}
              aria-label={t('adminA.filterGroupAria')}
            >
              <option value="">{t('adminA.allGroups')}</option>
              {groups.map((g) => (
                <option key={g.id} value={g.id}>
                  {g.name}
                </option>
              ))}
            </select>
            <select
              value={status}
              onChange={(e) => setParam('status', e.target.value)}
              className={forms.select}
              aria-label={t('adminA.filterStatusAria')}
            >
              <option value="">{t('adminA.allStatuses')}</option>
              <option value="active">{t('adminA.activeUsers')}</option>
              <option value="banned">{t('adminA.bannedUsers')}</option>
            </select>
            <select
              value={channel}
              onChange={(e) => setParam('channel', e.target.value)}
              className={forms.select}
              aria-label={t('adminA.filterChannelAria')}
            >
              <option value="">{t('adminA.allChannels')}</option>
              <option value="direct">{t('adminA.channelDirect')}</option>
              <option value="invite">{t('adminA.channelInvite')}</option>
              <option value="utm">{t('adminA.channelUtm')}</option>
              <option value="referer">{t('adminA.channelReferer')}</option>
            </select>
            <a
              className={styles.csvLink}
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
          </div>
        }
      />
      <AdminQueryGate query={users}>
        {(data) =>
          data.items.length === 0 ? (
            <EmptyState title={t('adminA.noMatchingUsers')} desc={t('adminA.noMatchingUsersDesc')} />
          ) : (
            <>
              <div className={styles.table}>
                <div className={`${styles.head} ${styles.row}`} role="row">
                  <span className={styles.alignStart}>{t('adminA.colUser')}</span>
                  <span className={styles.alignStart}>{t('adminA.colGroup')}</span>
                  <span className={styles.alignEnd}>{t('adminA.colImageCount')}</span>
                  <SortTh col="storage" label={t('adminA.colUsedStorage')} align="end" />
                  <SortTh col="bandwidth" label={t('adminA.colBandwidth')} align="end" />
                  <SortTh col="created" label={t('adminA.colRegistered')} align="start" />
                  <SortTh col="last_seen" label={t('adminA.colLastSeen')} align="start" />
                  <span className={styles.alignStart}>{t('adminA.colStatus')}</span>
                  <span className={styles.alignEnd}>{t('adminA.colActions')}</span>
                </div>
                {data.items.map((u) => {
                  const quota = groupQuota(u.group_id)
                  const bwQuota = groupBwQuota(u.group_id)
                  const usedBw = u.bandwidth_used_month ?? 0
                  const pct = quota > 0 ? Math.min(100, Math.round((u.used_storage / quota) * 100)) : 0
                  const bwPct = bwQuota > 0 ? Math.min(100, Math.round((usedBw / bwQuota) * 100)) : 0
                  return (
                    <div key={u.id} className={styles.row} role="row">
                      <div className={`${styles.userCell} ${styles.alignStart}`}>
                        <span className={styles.initial}>{(u.nickname || u.username).slice(0, 1)}</span>
                        <div className={styles.userMeta}>
                          <span className={styles.uname}>{u.username}</span>
                          <span className={styles.email} title={u.email}>
                            {u.email}
                            {u.email_verified === false && (
                              <span className={styles.unverified} title={t('adminA.emailVerifiedNo')}>
                                {' '}
                                · {t('adminA.emailVerifiedNo')}
                              </span>
                            )}
                          </span>
                        </div>
                      </div>
                      <select
                        className={`${styles.groupSel} ${styles.alignStart}`}
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
                      <span className={`${styles.mono} ${styles.alignEnd}`}>{u.image_count}</span>
                      <div className={`${styles.usageCell} ${styles.alignEnd}`}>
                        {quota > 0 && (
                          <span className={styles.usageBar} title={`${pct}%`}>
                            <span style={{ width: `${pct}%` }} />
                          </span>
                        )}
                        <span className={styles.usageText}>{formatBytes(u.used_storage)}</span>
                      </div>
                      <div className={`${styles.usageCell} ${styles.alignEnd}`}>
                        {bwQuota > 0 && (
                          <span className={styles.usageBar} title={`${bwPct}%`}>
                            <span style={{ width: `${bwPct}%` }} />
                          </span>
                        )}
                        <span className={styles.usageText}>
                          {formatBytes(usedBw)}
                          {u.bandwidth_period ? (
                            <span className={styles.period}> · {u.bandwidth_period}</span>
                          ) : null}
                        </span>
                      </div>
                      <span className={`${styles.time} ${styles.alignStart}`}>{formatDate(u.created_at)}</span>
                      <span className={`${styles.time} ${styles.alignStart}`}>
                        {u.last_seen_at ? formatDate(u.last_seen_at) : t('adminA.neverSeen')}
                      </span>
                      <span
                        className={[
                          styles.alignStart,
                          u.status === 'active' ? styles.stOk : styles.stErr,
                        ].join(' ')}
                      >
                        {u.status === 'active' ? t('adminA.statusActive') : t('adminA.statusBanned')}
                      </span>
                      <div className={`${styles.actions} ${styles.alignEnd}`}>
                        {u.status === 'active' ? (
                          <ArmedButton
                            className={styles.iconBtn}
                            armedClassName={styles.iconBtnArmed}
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
                            className={styles.iconBtn}
                            armedClassName={styles.iconBtnArmedOk}
                            title={t('adminA.unban')}
                            armedTitle={t('adminA.confirmUnban')}
                            armedChildren={t('adminA.confirmUnban')}
                            onConfirm={() => update.mutate({ id: u.id, body: { status: 'active' } })}
                          >
                            <span aria-hidden>○</span>
                          </ArmedButton>
                        )}
                        <ArmedButton
                          className={styles.iconBtn}
                          armedClassName={styles.iconBtnArmed}
                          title={t('adminA.resetPassword')}
                          armedTitle={t('adminA.confirmResetIcon')}
                          armedChildren={t('adminA.confirmResetIcon')}
                          onConfirm={() => setResetTarget(u)}
                        >
                          <span aria-hidden>⌁</span>
                        </ArmedButton>
                        <Link
                          className={styles.iconLink}
                          to={`/admin/images?user=${u.id}`}
                          title={t('adminA.viewImages')}
                          aria-label={t('adminA.viewImages')}
                        >
                          <span aria-hidden>▦</span>
                        </Link>
                      </div>
                    </div>
                  )
                })}
              </div>
              <div className={styles.footer}>
                <p className={styles.stat}>
                  {t('adminA.usersTotal', { total: data.total })}
                  <span className={styles.statSep}>·</span>
                  {t('adminA.usersPageHint', { page: data.page, limit: data.limit })}
                  <span className={styles.statSep}>·</span>
                  {t('adminA.sortBy')}: {sortLabel(t, sort)}
                </p>
                <Pager
                  page={page}
                  limit={data.limit}
                  total={data.total}
                  onPage={(p) => setParam('page', p > 1 ? String(p) : '')}
                />
              </div>
            </>
          )
        }
      </AdminQueryGate>
      <Modal open={resetTarget !== null} onClose={closeReset} width={400}>
        {resetTarget && (
          <div className={styles.resetBox}>
            {reset.data ? (
              <>
                <h2 className={styles.resetTitle}>{t('adminA.passwordGenerated')}</h2>
                <div className={styles.pw}>{reset.data.password}</div>
                <p className={styles.resetHint}>{t('adminA.passwordOnceHint')}</p>
                <div className={styles.resetBtns}>
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
                <h2 className={styles.resetTitle}>{t('adminA.resetPasswordTitle')}</h2>
                <p className={styles.resetHint}>
                  {t('adminA.resetPasswordHintBefore')}
                  <b>{resetTarget.username}</b>
                  {t('adminA.resetPasswordHintAfter')}
                </p>
                <div className={styles.resetBtns}>
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
