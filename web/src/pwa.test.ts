import { describe, it, expect, vi, beforeEach } from 'vitest'

// dev(import.meta.env.PROD 默认 false in vitest):registerSW 不调 register
describe('registerSW', () => {
  beforeEach(() => {
    vi.unstubAllGlobals()
  })

  it('dev 不注册 SW', async () => {
    const register = vi.fn(() => Promise.resolve({} as ServiceWorkerRegistration))
    vi.stubGlobal('navigator', { serviceWorker: { register } })
    const { registerSW } = await import('./pwa')
    registerSW()
    window.dispatchEvent(new Event('load'))
    expect(register).not.toHaveBeenCalled() // PROD=false
  })
})
