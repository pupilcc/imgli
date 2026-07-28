import { MutationCache, QueryClient } from '@tanstack/react-query'
import { t } from './i18n'
import { useGlobal } from './store'

/** 全局 QueryClient 工厂：queries 不重试；mutation 失败时全局 toast 兜底
 *（hook 级 onError 存根可跳过——见 api/hooks.ts 注释）。测试复用以获得同等语义。 */
export function createQueryClient() {
  return new QueryClient({
    defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } },
    mutationCache: new MutationCache({
      onError: (_e, _v, _c, mutation) => {
        if (mutation.options.onError) return
        useGlobal.getState().pushToast(t('errors.generic'))
      },
    }),
  })
}
