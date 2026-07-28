import { t } from '../i18n'
import { useGlobal } from '../store'

/** 复制文本并按全站规范弹 toast（所有复制操作必带反馈）。 */
export async function copyText(text: string, label: string): Promise<void> {
  const { pushToast } = useGlobal.getState()
  try {
    await navigator.clipboard.writeText(text)
    pushToast(t('common.copied', { label }))
  } catch {
    pushToast(t('common.copyFailed'))
  }
}
