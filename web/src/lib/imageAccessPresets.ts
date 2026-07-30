/** Shared expiry / max-views presets for upload options and image detail edit. */

/** value = expires_in seconds; 0 = never */
export const EXPIRY_PRESETS = [
  { key: 'never', sec: 0 },
  { key: '1h', sec: 3600 },
  { key: '1d', sec: 86400 },
  { key: '7d', sec: 604800 },
  { key: '30d', sec: 2592000 },
] as const

export type ExpiryKey = (typeof EXPIRY_PRESETS)[number]['key']

export const EXPIRY_LABEL_KEY: Record<ExpiryKey, string> = {
  never: 'upload.expiryNever',
  '1h': 'upload.expiry1h',
  '1d': 'upload.expiry1d',
  '7d': 'upload.expiry7d',
  '30d': 'upload.expiry30d',
}

/** max_views presets; 0 = unlimited */
export const MAX_VIEWS_PRESETS = [
  { key: 'unlimited', n: 0 },
  { key: '1', n: 1 },
  { key: '3', n: 3 },
  { key: '10', n: 10 },
] as const

export type MaxViewsKey = (typeof MAX_VIEWS_PRESETS)[number]['key']

export const MAX_VIEWS_LABEL_KEY: Record<MaxViewsKey, string> = {
  unlimited: 'upload.maxViewsUnlimited',
  '1': 'upload.maxViews1',
  '3': 'upload.maxViews3',
  '10': 'upload.maxViews10',
}
