import '@testing-library/jest-dom/vitest'

// 锁 vitest 默认 locale 为 zh，避免 jsdom navigator.language=en-US 让中文断言失败
localStorage.setItem('imgli-lang', 'zh')

// jsdom 无 matchMedia；store.initialTheme 依赖它
Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: (query: string): MediaQueryList =>
    ({
      matches: false,
      media: query,
      onchange: null,
      addEventListener() {},
      removeEventListener() {},
      dispatchEvent() {
        return false
      },
    }) as unknown as MediaQueryList,
})

// jsdom 不实现 Blob URL；上传队列用它生成本地缩略图预览
if (!URL.createObjectURL) {
  URL.createObjectURL = (() => 'blob:mock') as typeof URL.createObjectURL
}
if (!URL.revokeObjectURL) {
  URL.revokeObjectURL = (() => {}) as typeof URL.revokeObjectURL
}
