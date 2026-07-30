import { generateAccessPassword, STRONG_RE } from './password'

it('generateAccessPassword length and charset', () => {
  const a = generateAccessPassword(10)
  expect(a).toHaveLength(10)
  expect(a).toMatch(/^[A-Za-z0-9]+$/)
  // no ambiguous 0/O/1/I/l in alphabet
  expect(a).not.toMatch(/[0O1Il]/)
  const b = generateAccessPassword(10)
  // extremely unlikely equal; allow rare flake by only checking shape of b
  expect(b).toHaveLength(10)
})

it('STRONG_RE still works', () => {
  expect(STRONG_RE.test('passw0rd')).toBe(true)
  expect(STRONG_RE.test('short')).toBe(false)
})
