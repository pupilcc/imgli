import { useT } from '../../../i18n'
import { Button } from '../../../ui/Button'
import { EmptyState } from '../../../ui/EmptyState'

export function AdminError({ onRetry }: { onRetry(): void }) {
  const { t } = useT()
  return (
    <EmptyState badge="ERR" title={t('adminB.loadFailed')} desc={t('adminB.loadFailedDesc')}>
      <Button variant="secondary" onClick={onRetry}>
        {t('common.retry')}
      </Button>
    </EmptyState>
  )
}
