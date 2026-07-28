import { useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router'
import { useAdminGroups, useAdminUsers, useResetAdminPassword, useUpdateAdminUser } from '../../../api/adminHooks'
import { useSession } from '../../../api/hooks'
import type { AdminUser } from '../../../api/types'
import { useT } from '../../../i18n'
import { copyText } from '../../../lib/copy'
import { formatBytes } from '../../../lib/format'
import { useDebounced } from '../../../lib/useDebounced'
import { PageHeader } from '../../../shell/PageHeader'
import { Button } from '../../../ui/Button'
import { EmptyState } from '../../../ui/EmptyState'
import { InlineConfirm } from '../../../ui/InlineConfirm'
import { Modal } from '../../../ui/Modal'
import { Skeleton } from '../../../ui/Skeleton'
import { AdminError } from '../ui/AdminError'
import { Pager } from '../ui/Pager'
import styles from './UsersPage.module.css'

export function UsersPage() {
  const { t } = useT()
  const [params, setParams] = useSearchParams()
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

  const setParam = (key: string, value: string) => {
    setParams((p) => {
      const n = new URLSearchParams(p)
      if (value) n.set(key, value)
      else n.delete(key)
      if (key !== 'page') n.delete('page')
      return n
    })
  }

  const { data: me } = useSession()
  const users = useAdminUsers({ q: q || undefined, group, status: status || undefined, page })
  const groupsQ = useAdminGroups()
  const update = useUpdateAdminUser()
  const reset = useResetAdminPassword()
  const [resetTarget, setResetTarget] = useState<AdminUser | null>(null)

  const groups = groupsQ.data?.items ?? []
  const groupName = (id: number) => groups.find((g) => g.id === id)?.name ?? `#${id}`
  const groupQuota = (id: number) => groups.find((g) => g.id === id)?.storage_quota ?? 0

  const closeReset = () => {
    setResetTarget(null)
    reset.reset()
  }

  return (
    <div>
      <PageHeader
        kicker="USERS"
        title={t('adminA.usersTitle')}
        extra={
          <div className={styles.filters}>
            <div className={styles.search}>
              <span className={styles.searchGlyph}>⌕</span>
              <input
                value={input}
                onChange={(e) => setInput(e.target.value)}
                placeholder={t('adminA.searchUsersPlaceholder')}
              />
            </div>
            <select value={group ?? ''} onChange={(e) => setParam('group', e.target.value)} className={styles.select} aria-label={t('adminA.filterGroupAria')}>
              <option value="">{t('adminA.allGroups')}</option>
              {groups.map((g) => (
                <option key={g.id} value={g.id}>
                  {g.name}
                </option>
              ))}
            </select>
            <select value={status} onChange={(e) => setParam('status', e.target.value)} className={styles.select} aria-label={t('adminA.filterStatusAria')}>
              <option value="">{t('adminA.allStatuses')}</option>
              <option value="active">{t('adminA.activeUsers')}</option>
              <option value="banned">{t('adminA.bannedUsers')}</option>
            </select>
          </div>
        }
      />
      {users.isError ? (
        <AdminError onRetry={() => users.refetch()} />
      ) : !users.data ? (
        <Skeleton height={220} />
      ) : users.data.items.length === 0 ? (
        <EmptyState title={t('adminA.noMatchingUsers')} desc={t('adminA.noMatchingUsersDesc')} />
      ) : (
        <>
          <div className={styles.table}>
            <div className={`${styles.head} ${styles.row}`}>
              <span>{t('adminA.colUser')}</span>
              <span>{t('adminA.colEmail')}</span>
              <span>{t('adminA.colEmailVerified')}</span>
              <span>{t('adminA.colGroup')}</span>
              <span>{t('adminA.colImageCount')}</span>
              <span>{t('adminA.colUsedStorage')}</span>
              <span>{t('adminA.colStatus')}</span>
              <span />
            </div>
            {users.data.items.map((u) => {
              const quota = groupQuota(u.group_id)
              const pct = quota > 0 ? Math.min(100, Math.round((u.used_storage / quota) * 100)) : 0
              return (
                <div key={u.id} className={styles.row}>
                  <div className={styles.userCell}>
                    <span className={styles.initial}>{(u.nickname || u.username).slice(0, 1)}</span>
                    <span className={styles.uname}>{u.username}</span>
                  </div>
                  <span className={styles.email}>{u.email}</span>
                  <span className={u.email_verified ? styles.stOk : styles.stErr}>
                    {u.email_verified ? t('adminA.emailVerifiedYes') : t('adminA.emailVerifiedNo')}
                  </span>
                  <select
                    className={styles.groupSel}
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
                  <span className={styles.mono}>{u.image_count}</span>
                  <div className={styles.usageCell}>
                    {quota > 0 && (
                      <span className={styles.usageBar}>
                        <span style={{ width: `${pct}%` }} />
                      </span>
                    )}
                    <span className={styles.usageText}>{formatBytes(u.used_storage)}</span>
                  </div>
                  <span className={u.status === 'active' ? styles.stOk : styles.stErr}>
                    {u.status === 'active' ? t('adminA.statusActive') : t('adminA.statusBanned')}
                  </span>
                  <div className={styles.actions}>
                    {u.status === 'active' ? (
                      <InlineConfirm
                        label={t('adminA.ban')}
                        confirmLabel={t('adminA.confirmBan')}
                        disabled={me?.id === u.id}
                        onConfirm={() => update.mutate({ id: u.id, body: { status: 'banned' } })}
                      />
                    ) : (
                      <Button variant="secondary" onClick={() => update.mutate({ id: u.id, body: { status: 'active' } })}>
                        {t('adminA.unban')}
                      </Button>
                    )}
                    <Button variant="secondary" onClick={() => setResetTarget(u)}>
                      {t('adminA.resetPassword')}
                    </Button>
                    <Link className={styles.viewLink} to={`/admin/images?user=${u.id}`}>
                      {t('adminA.viewImages')}
                    </Link>
                  </div>
                </div>
              )
            })}
          </div>
          <p className={styles.stat}>
            {t('adminA.usersTotal', { total: users.data.total })}
          </p>
          <Pager page={page} limit={users.data.limit} total={users.data.total} onPage={(p) => setParam('page', p > 1 ? String(p) : '')} />
        </>
      )}
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
