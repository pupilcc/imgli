import { useT } from '../../../i18n'
import { Button } from '../../../ui/Button'
import styles from './Pager.module.css'

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
    <div className={styles.pager}>
      <Button variant="secondary" disabled={page <= 1} onClick={() => onPage(page - 1)}>
        {t('adminB.prevPage')}
      </Button>
      <span className={styles.stat}>
        {t('adminB.pageStat', { page, pages })}
      </span>
      <Button variant="secondary" disabled={page >= pages} onClick={() => onPage(page + 1)}>
        {t('adminB.nextPage')}
      </Button>
    </div>
  )
}
