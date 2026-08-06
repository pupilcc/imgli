import { useState } from 'react'
import { Link } from 'react-router'
import {
  useAdminLogs,
  useAdminRefererImages,
  useAdminStats,
  useCheckSystemUpdate,
  useSystemUpgrade,
  useSystemVersion,
} from '../../../api/adminHooks'
import { useT } from '../../../i18n'
import { useGlobal } from '../../../store'
import { Button } from '../../../ui/Button'
import { formatBytes, formatDate } from '../../../lib/format'
import { PageHeader } from '../../../shell/PageHeader'
import { Skeleton } from '../../../ui/Skeleton'
import { AdminQueryGate } from '../ui/AdminQueryGate'
import { ACTION_LABELS, dotColor } from '../ui/auditActions'
import { TrendChart } from './TrendChart'

type RefWindow = 7 | 30

const cardClass = 'rounded-sm border border-border bg-surface p-4'
const cardLabelClass = 'mb-2.5 font-mono text-2xs tracking-[0.1em] text-muted'
const cardValueClass = 'text-[22px] font-extrabold tracking-tight'
const cardsClass = 'mb-3.5 grid grid-cols-2 gap-3.5 min-[901px]:grid-cols-4'
const panelsClass = 'mb-3.5 grid grid-cols-1 gap-3.5 min-[901px]:grid-cols-[1.6fr_1fr]'
const panelClass = 'min-w-0 rounded-sm border border-border bg-surface p-[18px]'
const panelHeadClass = 'mb-4 flex items-baseline justify-between font-mono text-2xs tracking-[0.1em] text-muted'
const eventEmptyClass = 'text-sm-plus text-muted'
const sectionLabelClass = 'my-1 mb-2.5 font-mono text-2xs uppercase tracking-[0.12em] text-muted'
const winBtnClass =
  'cursor-pointer rounded-sm border border-border bg-transparent px-1.5 py-0.5 font-mono text-2xs text-muted'
const winActiveClass =
  'cursor-pointer rounded-sm border border-ink bg-transparent px-1.5 py-0.5 font-mono text-2xs font-bold text-ink'
const refTableClass =
  'w-full border-collapse text-sm-plus [&_th]:border-b [&_th]:border-border [&_th]:py-2 [&_th]:text-left [&_th]:font-mono [&_th]:text-2xs [&_th]:font-medium [&_th]:tracking-[0.08em] [&_th]:text-muted [&_td]:border-b [&_td]:border-border [&_td]:py-2 [&_td]:text-left [&_tr:last-child_td]:border-b-0'
const refCountClass = 'w-16 text-right font-mono tabular-nums'
const caveatClass = 'mt-3 mb-0 text-[11px] leading-snug text-muted'
const eventActorClass = 'font-mono text-muted'

export function DashboardPage() {
  const { t } = useT()
  const stats = useAdminStats()
  const logs = useAdminLogs({ limit: 8 })
  const verQ = useSystemVersion()
  const checkUpdate = useCheckSystemUpdate()
  const doUpgrade = useSystemUpgrade()
  const [refWindow, setRefWindow] = useState<RefWindow>(30)
  const [selectedHost, setSelectedHost] = useState<string | null>(null)
  const [updateMsg, setUpdateMsg] = useState<string | null>(null)
  const [latestTag, setLatestTag] = useState<string | null>(null)
  const hostImages = useAdminRefererImages(selectedHost, refWindow)

  const onCheckUpdate = () => {
    setUpdateMsg(null)
    setLatestTag(null)
    checkUpdate.mutate(undefined, {
      onSuccess: (r) => {
        if (r.error) {
          setUpdateMsg(t('adminA.updateCheckFailed', { err: r.error }))
          return
        }
        if (r.update_available) {
          setLatestTag(r.latest ?? null)
          setUpdateMsg(t('adminA.updateAvailable', { latest: r.latest ?? '?' }))
          if (r.release_url) {
            useGlobal.getState().pushToast(t('adminA.updateAvailableToast', { latest: r.latest ?? '' }))
          }
        } else {
          setUpdateMsg(t('adminA.updateUpToDate', { current: r.current }))
        }
      },
    })
  }

  const onUpgrade = () => {
    if (!latestTag) return
    if (!window.confirm(t('adminA.upgradeConfirm', { latest: latestTag }))) return
    doUpgrade.mutate(
      { confirm: true, tag: latestTag },
      {
        onSuccess: (r) => {
          if (r.mode === 'docker_blocked' || r.error) {
            setUpdateMsg(r.message || r.error || t('adminA.upgradeFailed'))
            return
          }
          setUpdateMsg(r.message || t('adminA.upgradeDone', { to: r.to ?? latestTag }))
          useGlobal.getState().pushToast(t('adminA.upgradeDoneToast'))
        },
      },
    )
  }

  return (
    <div>
      <PageHeader kicker="DASHBOARD" title={t('adminA.dashTitle')} />
      <div className="mb-4 flex flex-wrap items-center gap-x-4 gap-y-2.5 rounded-sm border border-border bg-soft px-4 py-3">
        <span className="text-[0.85rem] text-muted">{t('adminA.runningVersion')}</span>
        <code className="text-[0.95rem] font-semibold">{verQ.data?.current ?? '…'}</code>
        <Button variant="secondary" disabled={checkUpdate.isPending} onClick={onCheckUpdate}>
          {t('adminA.checkUpdate')}
        </Button>
        {latestTag && (
          <Button variant="primary" disabled={doUpgrade.isPending} onClick={onUpgrade}>
            {t('adminA.upgradeTo', { latest: latestTag })}
          </Button>
        )}
        <Link className="flex-[1_1_12rem] text-[0.9rem]" to="/admin/system">
          {t('adminA.systemTitle')} →
        </Link>
        {updateMsg && (
          <span className="flex-[1_1_12rem] text-[0.9rem]">
            {updateMsg}
            {checkUpdate.data?.release_url && checkUpdate.data.update_available && (
              <>
                {' '}
                <a href={checkUpdate.data.release_url} target="_blank" rel="noreferrer">
                  {t('adminA.releaseNotes')}
                </a>
              </>
            )}
          </span>
        )}
      </div>
      <AdminQueryGate query={stats}>
        {(data) => {
          const referers =
            refWindow === 7
              ? (data.top_referers ?? [])
              : (data.top_referers_30d ?? data.top_referers ?? [])
          return (
        <>
          <div className={sectionLabelClass}>{t('adminA.opsSection')}</div>
          <div className={cardsClass}>
            {[
              { label: t('adminA.usersCount'), value: String(data.users) },
              {
                label: t('adminA.signups30dTotal'),
                value: String((data.signups_30d ?? []).reduce((a, d) => a + d.count, 0)),
              },
              {
                label: t('adminA.bandwidthMonth'),
                value: formatBytes(data.bandwidth_used_month ?? 0),
              },
              { label: t('adminA.totalStorage'), value: formatBytes(data.storage) },
            ].map((c) => (
              <div key={c.label} className={cardClass}>
                <div className={cardLabelClass}>{c.label}</div>
                <div className={cardValueClass}>{c.value}</div>
              </div>
            ))}
          </div>
          {(data.bandwidth_top_users?.length ?? 0) > 0 && (
            <div className="-mt-1 mb-4 flex flex-wrap items-center gap-2 text-xs">
              <span className="font-mono text-2xs tracking-[0.08em] text-muted">{t('adminA.bandwidthTopUsers')}</span>
              {data.bandwidth_top_users!.map((u) => (
                <Link
                  key={u.user_id}
                  className="inline-flex items-center gap-1.5 rounded-sm border border-border bg-surface px-2 py-1 text-inherit no-underline hover:border-muted"
                  to={`/admin/users?q=${encodeURIComponent(u.username)}`}
                >
                  {u.username}
                  <span className="font-mono text-2xs text-muted">{formatBytes(u.used)}</span>
                </Link>
              ))}
            </div>
          )}

          <div className={sectionLabelClass}>{t('adminA.inventorySection')}</div>
          <div className={cardsClass}>
            {[
              { label: t('adminA.imagesCount'), value: String(data.images) },
              { label: t('adminA.todayUploads'), value: String(data.today_uploads) },
              { label: t('adminA.pendingImages'), value: String(data.pending_images ?? 0) },
              { label: t('adminA.rejectedImages'), value: String(data.rejected_images ?? 0) },
              { label: t('adminA.tasksPending'), value: String(data.tasks_pending ?? 0) },
              { label: t('adminA.tasksRunning'), value: String(data.tasks_running ?? 0) },
            ].map((c) => (
              <div key={c.label} className={cardClass}>
                <div className={cardLabelClass}>{c.label}</div>
                <div className={cardValueClass}>{c.value}</div>
              </div>
            ))}
          </div>

          <div className={panelsClass}>
            <div className={panelClass}>
              <div className={panelHeadClass}>
                <span>{t('adminA.signupsTrend30d')}</span>
                <span>{t('adminA.unitUsersPerDay')}</span>
              </div>
              <TrendChart daily={data.signups_30d ?? []} days={30} />
            </div>
            <div className={panelClass}>
              <div className={panelHeadClass}>
                <span>{t('adminA.signupChannels')}</span>
              </div>
              {(data.signup_channels_30d ?? []).length === 0 ? (
                <div className={eventEmptyClass}>{t('adminA.noSignups')}</div>
              ) : (
                <table className={refTableClass}>
                  <thead>
                    <tr>
                      <th>{t('adminA.channel')}</th>
                      <th>{t('adminA.hits')}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {(data.signup_channels_30d ?? []).map((r) => (
                      <tr key={r.channel}>
                        <td>{channelLabel(t, r.channel)}</td>
                        <td className={refCountClass}>{r.count}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </div>
          </div>

          <div className={panelsClass}>
            <div className={panelClass}>
              <div className={panelHeadClass}>
                <span>{t('adminA.uploadTrend30d')}</span>
                <span>{t('adminA.unitImagesPerDay')}</span>
              </div>
              <TrendChart daily={data.daily ?? []} />
            </div>
            <div className={panelClass}>
              <div className={panelHeadClass}>
                <span>{t('adminA.recentEvents')}</span>
              </div>
              <div className="flex flex-col gap-[13px]">
                {logs.data && logs.data.items.length === 0 && <div className={eventEmptyClass}>{t('adminA.noEvents')}</div>}
                {logs.data?.items.map((l) => (
                  <div key={l.id} className="flex items-start gap-2.5">
                    <span className="mt-[5px] h-1.5 w-1.5 flex-none rounded-full" style={{ background: dotColor(l.action) }} />
                    <div className="min-w-0">
                      <div className="text-sm-plus leading-snug">
                        {ACTION_LABELS[l.action] ?? l.action}
                        {l.actor_id != null && <span className={eventActorClass}> · #{l.actor_id}</span>}
                      </div>
                      <div className="mt-0.5 font-mono text-2xs text-muted">{formatDate(l.created_at)}</div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </div>

          <div className={panelsClass}>
            <div className={panelClass}>
              <div className={panelHeadClass}>
                <span>{refWindow === 7 ? t('adminA.traffic7d') : t('adminA.traffic30d')}</span>
                <span className="inline-flex gap-1">
                  <button type="button" className={refWindow === 7 ? winActiveClass : winBtnClass} onClick={() => setRefWindow(7)}>
                    7d
                  </button>
                  <button type="button" className={refWindow === 30 ? winActiveClass : winBtnClass} onClick={() => setRefWindow(30)}>
                    30d
                  </button>
                </span>
              </div>
              <TrendChart
                daily={(refWindow === 7
                  ? (data.traffic_7d ?? [])
                  : (data.traffic_30d ?? data.traffic_7d ?? [])
                ).map((d) => ({ date: d.date, count: d.views }))}
                days={refWindow}
              />
              <p className={caveatClass}>{t('adminA.originMeteringNote')}</p>
            </div>
            <div className={panelClass}>
              <div className={panelHeadClass}>
                <span>{t('adminA.topReferers')}</span>
                <span className="inline-flex gap-1">
                  <button type="button" className={refWindow === 7 ? winActiveClass : winBtnClass} onClick={() => setRefWindow(7)}>
                    7d
                  </button>
                  <button type="button" className={refWindow === 30 ? winActiveClass : winBtnClass} onClick={() => setRefWindow(30)}>
                    30d
                  </button>
                </span>
              </div>
              {referers.length === 0 ? (
                <div className={eventEmptyClass}>{t('adminA.noReferers')}</div>
              ) : (
                <table className={refTableClass}>
                  <thead>
                    <tr>
                      <th>{t('adminA.domain')}</th>
                      <th>{t('adminA.hits')}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {referers.map((r) => (
                      <tr
                        key={r.host}
                        className="cursor-pointer hover:bg-soft"
                        onClick={() => setSelectedHost(r.host === selectedHost ? null : r.host)}
                      >
                        <td>
                          {r.host}
                          {r.suspect ? (
                            <span className="ml-1.5 rounded-sm border border-warn px-1 font-mono text-[9px] tracking-[0.06em] text-warn">
                              {t('adminA.suspect')}
                            </span>
                          ) : null}
                        </td>
                        <td className={refCountClass}>{r.count}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
              <p className={caveatClass}>{t('adminA.originMeteringNote')}</p>
              {selectedHost && (
                <div className="mt-3.5 border-t border-dashed border-border pt-3">
                  <div className={panelHeadClass}>
                    <span>
                      {t('adminA.refererImages')}: {selectedHost}
                    </span>
                    <button type="button" className={winBtnClass} onClick={() => setSelectedHost(null)}>
                      ×
                    </button>
                  </div>
                  {hostImages.isLoading ? (
                    <Skeleton height={80} />
                  ) : (hostImages.data?.items ?? []).length === 0 ? (
                    <div className={eventEmptyClass}>{t('adminA.noRefererImages')}</div>
                  ) : (
                    <table className={refTableClass}>
                      <thead>
                        <tr>
                          <th>{t('adminA.imageKey')}</th>
                          <th>{t('adminA.hits')}</th>
                        </tr>
                      </thead>
                      <tbody>
                        {hostImages.data!.items.map((it) => (
                          <tr key={it.key}>
                            <td>
                              <Link to={`/admin/images?q=${encodeURIComponent(it.key)}`}>{it.key}</Link>
                              {it.name ? <span className={eventActorClass}> · {it.name}</span> : null}
                            </td>
                            <td className={refCountClass}>{it.count}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  )}
                </div>
              )}
            </div>
          </div>
        </>
          )
        }}
      </AdminQueryGate>
    </div>
  )
}

function channelLabel(t: (k: string) => string, ch: string): string {
  switch (ch) {
    case 'direct':
      return t('adminA.channelDirect')
    case 'invite':
      return t('adminA.channelInvite')
    case 'utm':
      return t('adminA.channelUtm')
    case 'referer':
      return t('adminA.channelReferer')
    default:
      return t('adminA.channelUnknown')
  }
}
