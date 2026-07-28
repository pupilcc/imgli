export type Lang = 'zh' | 'en'

export function detectLang(): Lang {
  const saved = localStorage.getItem('imgli-lang')
  if (saved === 'zh' || saved === 'en') return saved
  return navigator.language?.toLowerCase().startsWith('zh') ? 'zh' : 'en'
}

export function setHtmlLang(l: Lang): void {
  document.documentElement.lang = l === 'zh' ? 'zh-CN' : 'en'
}
