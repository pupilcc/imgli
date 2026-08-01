import { useState } from 'react'
import { useNavigate } from 'react-router'
import { useT } from '../../i18n'
import { Button } from '../../ui/Button'
import { StepGuide } from '../../ui/StepGuide'

const KEY = 'imgli_onboarding_dismissed'

function wasDismissed(): boolean {
  try {
    return typeof localStorage !== 'undefined' && localStorage.getItem(KEY) === '1'
  } catch {
    return false
  }
}

export function FirstRunOnboarding({ show }: { show: boolean }) {
  const { t } = useT()
  const navigate = useNavigate()
  const [dismissed, setDismissed] = useState(wasDismissed)

  if (!show || dismissed) return null

  function dismiss() {
    try {
      localStorage.setItem(KEY, '1')
    } catch {
      /* ignore */
    }
    setDismissed(true)
  }

  return (
    <StepGuide
      id="imgli-first-run"
      data-testid="first-run-onboarding"
      kicker={t('upload.onboardingKicker')}
      steps={[t('upload.onboardingStep1'), t('upload.onboardingStep2'), t('upload.onboardingStep3')]}
      actions={
        <>
          <Button variant="primary" type="button" onClick={() => navigate('/settings')}>
            {t('upload.onboardingCta')}
          </Button>
          <Button variant="ghost" type="button" onClick={dismiss}>
            {t('upload.onboardingSkip')}
          </Button>
        </>
      }
    />
  )
}
