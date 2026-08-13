import { clsx, type ClassValue } from 'clsx'
import { extendTailwindMerge } from 'tailwind-merge'

/*
 * 自定义字号 token（tokens.css @theme 的 --text-2xs / --text-xs-plus / --text-sm-plus）
 * 不匹配 tailwind-merge 的 t-shirt 尺寸规则，默认会被归进兜底的 text-color 组，
 * 于是吃掉同一次调用里的 text-muted / text-btn-text 等颜色类（反之亦然）。
 * 往 @theme 新增 --text-* token 时，必须同步加进下面的数组。
 */
const twMerge = extendTailwindMerge({
  extend: { classGroups: { 'font-size': [{ text: ['2xs', 'xs-plus', 'sm-plus'] }] } },
})

/** Merge Tailwind classes; later args win on conflict. */
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}
