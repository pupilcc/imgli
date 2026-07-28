const VERSION = 'imgli-v1' // bump 即失效旧缓存
const SHELL = ['/'] // 应用壳(index.html),导航离线回落用

self.addEventListener('install', (e) => {
  e.waitUntil(caches.open(VERSION).then((c) => c.addAll(SHELL)))
  self.skipWaiting()
})

self.addEventListener('activate', (e) => {
  e.waitUntil(
    caches
      .keys()
      .then((keys) => Promise.all(keys.filter((k) => k !== VERSION).map((k) => caches.delete(k))))
      .then(() => self.clients.claim()),
  )
})

self.addEventListener('fetch', (e) => {
  const req = e.request
  const url = new URL(req.url)
  // 仅同源 GET 拦截;跨源/非 GET 直通
  if (req.method !== 'GET' || url.origin !== self.location.origin) return
  // 动态/大图/鉴权路径直通不缓存
  if (/^\/(api|i|t|avatar)\//.test(url.pathname)) return
  // 导航(HTML):network-first,成功更新壳缓存 '/',离线回落缓存壳
  if (req.mode === 'navigate') {
    e.respondWith(
      fetch(req)
        .then((res) => {
          const copy = res.clone()
          caches
            .open(VERSION)
            .then((c) => c.put('/', copy))
            .catch(() => {})
          return res
        })
        .catch(() => caches.match('/').then((r) => r || caches.match(req))),
    )
    return
  }
  // 静态资源(hashed 不变)cache-first
  if (/^\/(assets|pwa|brand)\//.test(url.pathname)) {
    e.respondWith(
      caches.match(req).then(
        (hit) =>
          hit ||
          fetch(req).then((res) => {
            const copy = res.clone()
            caches
              .open(VERSION)
              .then((c) => c.put(req, copy))
              .catch(() => {})
            return res
          }),
      ),
    )
    return
  }
  // 其余:network,失败回落缓存
  e.respondWith(fetch(req).catch(() => caches.match(req)))
})
