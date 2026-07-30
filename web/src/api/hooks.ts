import { useMutation, useQuery, useQueryClient, useInfiniteQuery } from '@tanstack/react-query'
import { api, post, del, patch } from './client'
import { useGlobal } from '../store'
import type {
  Album,
  Quota,
  User,
  ImageDetail,
  ImageStats,
  BatchResult,
  ImagesPage,
  TrashPage,
  ApiToken,
  PublicConfig,
  Preferences,
  PolicyOption,
  DiscoverPage,
  PublicProfileData,
  ShareImage,
} from './types'

export const sessionKey = ['session'] as const

export function useSession() {
  return useQuery({
    queryKey: sessionKey,
    queryFn: () => api<User>('/auth/session'),
    retry: false,
    // 未登录（无 data）时不要在每次新增订阅者挂载（如游客态 UploadPage 与
    // RequireAuthOrGuest 同时观察此 query）时都重新发起探测——否则 401 结果
    // 因 data===undefined 被视为“需要挂载即抓取”，与门禁组件的 isLoading 互相
    // 触发挂载/卸载，形成死循环。会话状态变化改走登录/登出 mutation 的
    // setQueryData/clear，无需靠自动重探测。
    retryOnMount: false,
    staleTime: 5 * 60_000,
  })
}

export function useQuota(enabled = true) {
  return useQuery({ queryKey: ['quota'], queryFn: () => api<Quota>('/user/quota'), enabled })
}

export function useAlbums(enabled = true) {
  return useQuery({ queryKey: ['albums'], queryFn: () => api<{ items: Album[] }>('/albums'), enabled })
}

/** 公开站点配置(无需登录)。游客模式路由与登录/注册页据此渲染。 */
export function useConfig() {
  return useQuery({
    queryKey: ['config'],
    queryFn: () => api<PublicConfig>('/config'),
    retry: false,
    staleTime: 5 * 60_000,
  })
}

export function useLogin() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: { account: string; password: string }) => post<User>('/auth/login', body),
    onSuccess: (user) => qc.setQueryData(sessionKey, user),
    // 调用方（AuthPage）已用局部 onError 展示行内错误文案；hook 级 onError 存根用于让全局兜底 toast 识别并跳过。
    onError: () => {},
  })
}

export type RegisterBody = {
  username: string
  email: string
  password: string
  invite_code?: string
  utm_source?: string
  utm_medium?: string
  utm_campaign?: string
  referer_host?: string
}

export function useRegister() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: RegisterBody) => post<User>('/auth/register', body),
    onSuccess: (user) => qc.setQueryData(sessionKey, user),
    onError: () => {},
  })
}

export function useLogout() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => post('/auth/logout'),
    onSuccess: () => qc.clear(),
  })
}

export interface ImagesFilter {
  q: string
  format: 'ALL' | 'PNG' | 'JPG' | 'GIF' | 'WEBP'
  album: 'all' | 'none' | number
  visibility: 'all' | 'public' | 'private'
  sort: 'date' | 'size' | 'name'
}

export const defaultFilter: ImagesFilter = { q: '', format: 'ALL', album: 'all', visibility: 'all', sort: 'date' }

function imagesQuery(f: ImagesFilter, cursor: string): string {
  const p = new URLSearchParams()
  if (f.q) p.set('q', f.q)
  if (f.format !== 'ALL') p.set('format', f.format)
  if (f.album === 'none') p.set('album', 'none')
  else if (f.album !== 'all') p.set('album', String(f.album))
  if (f.visibility !== 'all') p.set('visibility', f.visibility)
  if (f.sort !== 'date') p.set('sort', f.sort)
  if (cursor) p.set('cursor', cursor)
  p.set('limit', '24')
  return p.toString()
}

export function useImages(f: ImagesFilter) {
  return useInfiniteQuery({
    queryKey: ['images', f],
    queryFn: ({ pageParam }) => api<ImagesPage>(`/images?${imagesQuery(f, pageParam)}`),
    initialPageParam: '',
    getNextPageParam: (last) => last.next_cursor || undefined,
  })
}

/** 广场公开流；plaza 关闭时后端 404，不重试。 */
export function usePlaza(sort: 'new' | 'hot') {
  return useInfiniteQuery({
    queryKey: ['plaza', sort],
    queryFn: ({ pageParam }) =>
      api<DiscoverPage>(`/plaza?sort=${sort}&cursor=${encodeURIComponent(pageParam)}&limit=24`),
    initialPageParam: '',
    getNextPageParam: (last) => last.next_cursor || undefined,
    retry: false,
  })
}

/** 公开用户主页；用户不存在/未公开时后端 404。 */
export function useUserPublic(username: string) {
  return useQuery({
    queryKey: ['u', username],
    queryFn: () => api<{ user: PublicProfileData }>(`/u/${encodeURIComponent(username)}`),
    retry: false,
    enabled: !!username,
  })
}

/** 公开用户图库分页。 */
export function useUserImages(username: string, sort: 'new' | 'hot') {
  return useInfiniteQuery({
    queryKey: ['u-images', username, sort],
    queryFn: ({ pageParam }) =>
      api<DiscoverPage>(
        `/u/${encodeURIComponent(username)}/images?sort=${sort}&cursor=${encodeURIComponent(pageParam)}&limit=24`,
      ),
    initialPageParam: '',
    getNextPageParam: (last) => last.next_cursor || undefined,
    retry: false,
    enabled: !!username,
  })
}

export function useImageDetail(key: string | null) {
  return useQuery({
    queryKey: ['image', key],
    enabled: !!key,
    queryFn: () => api<ImageDetail>(`/images/${key}`),
  })
}

/** Public share landing meta (no auth). */
export function useShareImage(key: string | null) {
  return useQuery({
    queryKey: ['share', key],
    enabled: !!key,
    retry: false,
    queryFn: () => api<ShareImage>(`/s/${key}`),
  })
}

/** Unlock password-protected share; sets HttpOnly cookie on success. */
export function useUnlockShare() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ key, password }: { key: string; password: string }) =>
      post<ShareImage>(`/s/${key}/unlock`, { password }),
    onSuccess: (data, v) => {
      qc.setQueryData(['share', v.key], data)
      qc.invalidateQueries({ queryKey: ['share', v.key] })
    },
  })
}

/** 详情弹窗 ACCESS 区块;key 为 null(弹窗关闭)不发请求。 */
export function useImageStats(key: string | null) {
  return useQuery({
    queryKey: ['image-stats', key],
    enabled: !!key,
    queryFn: () => api<ImageStats>(`/images/${key}/stats`),
  })
}

export function useUpdateImage() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({
      key,
      body,
    }: {
      key: string
      body: {
        name?: string
        visibility?: string
        album_id?: number | null
        expires_in?: number
        slug?: string | null
        max_views?: number
        access_password?: string
      }
    }) => patch<ImageDetail>(`/images/${key}`, body),
    onSuccess: (_d, v) => {
      qc.invalidateQueries({ queryKey: ['images'] })
      qc.invalidateQueries({ queryKey: ['image', v.key] })
      qc.invalidateQueries({ queryKey: ['albums'] })
    },
  })
}

export function useDeleteImage() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (key: string) => del(`/images/${key}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['images'] })
      qc.invalidateQueries({ queryKey: ['quota'] })
      qc.invalidateQueries({ queryKey: ['albums'] })
    },
  })
}

export function useBatchImages() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: { action: 'delete' | 'visibility' | 'move'; keys: string[]; visibility?: string; album_id?: number | null }) =>
      post<{ results: BatchResult[] }>('/images/batch', body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['images'] })
      qc.invalidateQueries({ queryKey: ['quota'] })
      qc.invalidateQueries({ queryKey: ['albums'] })
    },
    // BatchBar 的 run() 自行 try/catch 并按分块结果 toast；hook 级存根让全局兜底识别并跳过，避免双 toast。
    onError: () => {},
  })
}

export function useCreateAlbum() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: { name: string; visibility: string }) =>
      post<Pick<Album, 'id' | 'name' | 'visibility'>>('/albums', body),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['albums'] }),
  })
}

export function useUpdateAlbum() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, body }: { id: number; body: { name?: string; visibility?: string } }) =>
      patch(`/albums/${id}`, body),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['albums'] }),
  })
}

export function useDeleteAlbum() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, withImages }: { id: number; withImages: boolean }) =>
      del(`/albums/${id}?with_images=${withImages}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['albums'] })
      qc.invalidateQueries({ queryKey: ['images'] })
      qc.invalidateQueries({ queryKey: ['trash'] })
    },
  })
}

export function useTrash() {
  return useInfiniteQuery({
    queryKey: ['trash'],
    queryFn: ({ pageParam }) =>
      api<TrashPage>(`/trash?limit=24${pageParam ? `&cursor=${encodeURIComponent(pageParam)}` : ''}`),
    initialPageParam: '',
    getNextPageParam: (last) => last.next_cursor || undefined,
  })
}

export function useRestoreImage() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (key: string) => post(`/trash/${key}/restore`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['trash'] })
      qc.invalidateQueries({ queryKey: ['images'] })
    },
  })
}

export function usePurgeImage() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (key: string) => del(`/trash/${key}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['trash'] })
      qc.invalidateQueries({ queryKey: ['quota'] })
    },
  })
}

export function useEmptyTrash() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => del<{ purged: number }>('/trash'),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['trash'] })
      qc.invalidateQueries({ queryKey: ['quota'] })
    },
  })
}

export function useTokens() {
  return useQuery({ queryKey: ['tokens'], queryFn: () => api<ApiToken[]>('/user/tokens') })
}

export function useCreateToken() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: { name: string; scope: 'upload' | 'full' }) => post<ApiToken>('/user/tokens', body),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['tokens'] }),
  })
}

export function useRevokeToken() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: number) => del(`/user/tokens/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['tokens'] }),
  })
}

export function useUpdateProfile() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: { nickname?: string; public_profile?: boolean }) => patch('/user/profile', body),
    onSuccess: () => qc.invalidateQueries({ queryKey: sessionKey }),
  })
}

export function useChangePassword() {
  return useMutation({
    mutationFn: (body: { old_password: string; new_password: string }) => patch('/user/password', body),
    // 页面用 call 级 onError 行内展示；hook 级存根跳过全局兜底 toast（见 MutationCache 守卫）
    onError: () => {},
  })
}

export function useChangeEmail() {
  return useMutation({
    mutationFn: (body: { password: string; new_email: string }) => post('/user/email/change', body),
    onError: () => {},
  })
}

export function useConfirmChangeEmail() {
  return useMutation({
    mutationFn: (body: { token: string }) => post('/auth/confirm-change-email', body),
    onError: () => {},
  })
}

export function useForgotPassword() {
  return useMutation({
    mutationFn: (body: { email: string }) => post('/auth/forgot-password', body),
    // 页面用 onSettled 恒显成功文案；hook 级存根跳过全局兜底 toast
    onError: () => {},
  })
}

export function useResetPassword() {
  return useMutation({
    mutationFn: (body: { token: string; password: string }) => post('/auth/reset-password', body),
    // 页面用 call 级 onError 行内展示；hook 级存根跳过全局兜底 toast
    onError: () => {},
  })
}

export function useVerifyEmail() {
  return useMutation({
    mutationFn: (body: { token: string }) => post('/auth/verify-email', body),
    // 页面用 call 级 onError 展示失败态；hook 级存根跳过全局兜底 toast
    onError: () => {},
  })
}

export function useResendVerification() {
  return useMutation({
    mutationFn: () => post('/auth/resend-verification'),
    // 失败走全局 toast 兜底；不加 onError 存根
  })
}

export function useUpdatePreferences() {
  const qc = useQueryClient()
  return useMutation({
    // 偏好为全量替换:任何保存都注入当前 UI 语言兜底,防省略 lang 清空已存语言偏好
    // (codex 基建评审 F1)。调用方显式给 lang(如 LangToggle) 则用其值。
    mutationFn: (body: Preferences) =>
      patch('/user/preferences', { ...body, lang: body.lang || useGlobal.getState().lang }),
    onSuccess: () => qc.invalidateQueries({ queryKey: sessionKey }),
  })
}

/** 当前用户可选存储策略;仅 >1 时前端渲染选择器。 */
export function useUserPolicies(enabled = true) {
  return useQuery({
    queryKey: ['user-policies'],
    queryFn: () => api<PolicyOption[]>('/user/policies'),
    staleTime: 5 * 60_000,
    enabled,
  })
}

export function useUploadAvatar() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (file: File) => {
      const fd = new FormData()
      fd.append('file', file)
      return api('/user/avatar', { method: 'POST', body: fd })
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: sessionKey }),
  })
}

export function useDeleteAvatar() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => del('/user/avatar'),
    onSuccess: () => qc.invalidateQueries({ queryKey: sessionKey }),
  })
}

export function useUploadWatermark() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (file: File) => {
      const fd = new FormData()
      fd.append('file', file)
      return api('/user/watermark', { method: 'POST', body: fd })
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: sessionKey }),
  })
}

export function useDeleteWatermark() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => del('/user/watermark'),
    onSuccess: () => qc.invalidateQueries({ queryKey: sessionKey }),
  })
}

export function useDeleteAccount() {
  return useMutation({
    mutationFn: (body: { password: string }) => del('/user', body),
    // ProfileTab 危险区行内展示错误;hook 级存根跳过全局兜底 toast
    onError: () => {},
  })
}
