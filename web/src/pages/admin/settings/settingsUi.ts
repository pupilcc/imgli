/** Shared Tailwind class strings for admin system settings tabs. */
export const s = {
  form: 'mt-2 mb-0 flex max-w-[640px] flex-col gap-5 pb-[72px]',
  tabs: 'sticky top-0 z-[5] isolate mx-[-2px] mt-0 mb-0 flex flex-wrap gap-x-[18px] gap-y-1 border-b border-border bg-bg pt-1 shadow-[0_8px_14px_-12px_rgba(0,0,0,0.22)]',
  tab: 'cursor-pointer whitespace-nowrap border-0 bg-transparent px-0.5 pb-[11px] text-[13px] font-semibold text-muted hover:text-ink',
  tabActive: 'text-ink shadow-[inset_0_-2px_0_var(--text)]',
  section: 'flex flex-col gap-3.5 rounded-lg border border-border bg-surface p-5',
  h2: 'm-0 text-[15px] font-semibold',
  h2Row: 'flex items-center justify-between',
  field: 'flex flex-col gap-1.5',
  label: 'text-[13px] text-muted',
  hint: 'text-xs text-muted',
  hintWarn: 'text-xs leading-[1.45] text-warn',
  textarea:
    'min-h-[88px] w-full resize-y rounded-sm border border-border bg-surface px-3 py-2.5 font-inherit text-[13.5px] text-ink outline-none focus:border-muted',
  subTabs: 'mb-1 -mt-1 flex flex-wrap gap-1',
  subTab:
    'cursor-pointer rounded-full border border-border bg-surface px-3 py-[5px] text-xs font-semibold text-muted hover:text-ink',
  subTabActive: 'border-btn bg-btn text-btn-text hover:text-btn-text',
  localePair: 'grid grid-cols-1 gap-x-3 gap-y-2.5 min-[721px]:grid-cols-2',
  fold: 'overflow-hidden rounded-md border border-border bg-bg [&_summary]:list-none [&_summary]:cursor-pointer [&_summary]:select-none [&_summary]:px-3 [&_summary]:py-2.5 [&_summary]:text-[13px] [&_summary]:font-semibold [&_summary::-webkit-details-marker]:hidden',
  foldBody: 'flex flex-col gap-3 border-t border-border px-3 pt-3 pb-3',
  slotCard: 'flex flex-col gap-2.5 rounded-md border border-dashed border-border p-3.5',
  slotRow: 'flex flex-col gap-2 border-t border-border pt-1',
  slotActions: 'flex flex-wrap gap-2',
  sliderHead: 'flex items-center justify-between',
  row: 'flex items-center justify-between gap-3',
  slider: 'w-full',
  mono: 'text-[13px] tabular-nums',
  actions:
    'sticky bottom-0 z-[2] mx-[-4px] flex bg-linear-to-t from-bg from-70% to-transparent py-3',
  testRow: 'flex items-end gap-2.5',
  lexiconToolbar: 'flex flex-wrap gap-2',
  fileInput: 'hidden',
  select:
    'rounded-md border border-border bg-bg px-3 py-[9px] font-inherit text-[13.5px] text-ink outline-none focus:border-muted',
} as const
