import { t } from './index'
import { useGlobal } from '../store'

// 错误消息展示策略(前端主导 + 后端中文兜底):
// - zh 用户:后端 message 本就是中文且细分(如「不能封禁自己」),直接用,不丢具体信息(不回归)。
// - en 用户:后端 message 是中文对其无用,按信封 code 映射英文通用译文 errors.<code>,
//   无映射再回落后端 message(该 code 无英文映射时兜底)。
export function errorText(code: string | undefined, fallback: string): string {
  if (useGlobal.getState().lang === 'zh') return fallback
  if (!code) return fallback
  const key = `errors.${code}`
  const msg = t(key)
  return msg === key ? fallback : msg
}
