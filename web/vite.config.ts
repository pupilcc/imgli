/// <reference types="vitest/config" />
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'
import { configDefaults } from 'vitest/config'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    proxy: {
      '/api': { target: 'http://localhost:8686' },
      '^/i/': { target: 'http://localhost:8686' },
      '^/t/': { target: 'http://localhost:8686' },
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: './src/vitest.setup.ts',
    // module-level vi.mock()（如 queue.test.ts 的 uploader/api-client）跨 it() 持久，
    // 不清零会导致 toHaveBeenCalledTimes/not.toHaveBeenCalled 断言把前序用例的调用计入。
    clearMocks: true,
    exclude: [...configDefaults.exclude, 'e2e/**'],
  },
})
