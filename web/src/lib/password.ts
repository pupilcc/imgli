export const STRONG_RE = /^(?=.*[A-Za-z])(?=.*\d).{8,}$/

/** 图片访问口令：可读、可复制的随机串（非账号密码强度规则）。 */
const ACCESS_PW_ALPHABET = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789'

export function generateAccessPassword(length = 10): string {
  const n = Math.max(6, Math.min(32, length))
  const buf = new Uint8Array(n)
  crypto.getRandomValues(buf)
  let out = ''
  for (let i = 0; i < n; i++) {
    out += ACCESS_PW_ALPHABET[buf[i]! % ACCESS_PW_ALPHABET.length]
  }
  return out
}
