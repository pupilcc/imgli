import { Link } from 'react-router'
import { useT } from '../i18n'
import { Button } from '../ui/Button'
import { EmptyState } from '../ui/EmptyState'

export function NotFoundPage() {
  const { t } = useT()
  return (
    <div style={{ maxWidth: 760, margin: '0 auto', paddingTop: 96 }}>
      <EmptyState badge="404" title={t('common.notFound')} desc={t('common.notFoundDesc')}>
        <Link to="/">
          <Button variant="primary">{t('nav.backToUpload')}</Button>
        </Link>
      </EmptyState>
    </div>
  )
}
