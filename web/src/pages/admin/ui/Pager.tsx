import { useT } from '../../../i18n'
import { Button } from '../../../ui/Button'

interface Props {
  page: number
  limit: number
  total: number
  onPage(p: number): void
}

export function Pager({ page, limit, total, onPage }: Props) {
  const { t } = useT()
  const pages = Math.ceil(total / limit)
  if (pages <= 1) return null
  return (
    <div className="mt-3 flex items-center justify-end gap-2">
      <Button variant="secondary" disabled={page <= 1} onClick={() => onPage(page - 1)}>
        {t('adminB.prevPage')}
      </Button>
      <span className="font-mono text-xs-plus text-muted">{t('adminB.pageStat', { page, pages })}</span>
      <Button variant="secondary" disabled={page >= pages} onClick={() => onPage(page + 1)}>
        {t('adminB.nextPage')}
      </Button>
    </div>
  )
}
