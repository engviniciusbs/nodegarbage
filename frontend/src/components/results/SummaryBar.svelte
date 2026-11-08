<script lang="ts">
  import type { SummaryDTO } from '$lib/api'
  import { formatBytes } from '$lib/format'
  import { results, toggleFilter, NODE_MODULES_RULE, type ActiveFilter } from '$lib/stores/results.svelte'
  import { strings } from '$lib/strings'

  let { summary }: { summary: SummaryDTO } = $props()

  const label = (c: string) => (strings.category as Record<string, string>)[c] ?? c
  const figs = $derived([
    { id: 'safe', bytes: summary.safeBytes, text: strings.results.safeLabel, tone: 'text-safe text-[22px]' },
    { id: 'caution', bytes: summary.cautionBytes, text: strings.results.cautionLabel, tone: 'text-warn-text text-[22px]' },
    { id: 'keep', bytes: summary.keepBytes, text: strings.results.keepLabel, tone: 'text-keep text-[15px]' },
  ])
  // The fixed node_modules shortcut first (when the volume has any), then the categories.
  const chips = $derived([
    ...(results.shortcut && results.shortcut.count > 0
      ? [{ key: 'rule', bytes: results.shortcut.totalBytes,
           filter: { category: '', ruleId: NODE_MODULES_RULE, label: strings.results.nodeModules } }]
      : []),
    ...(summary.byCategory ?? []).map((c) => ({
      key: `cat:${c.category}`, bytes: c.bytes, filter: { category: c.category, ruleId: '', label: label(c.category) },
    })),
  ] as { key: string; bytes: number; filter: ActiveFilter }[])
  const isActive = (f: ActiveFilter) =>
    results.filter !== null && results.filter.category === f.category && results.filter.ruleId === f.ruleId
</script>

<section
  aria-label={strings.results.summaryLabel}
  class="mx-6 flex shrink-0 flex-wrap items-center justify-between gap-x-6 gap-y-2 rounded-md border border-stroke bg-card px-4 py-3.5"
>
  <div class="flex min-w-0 flex-wrap items-baseline gap-x-6 gap-y-1">
    {#each figs as f (f.id)}
      <p class="m-0 flex items-baseline gap-2" data-fig={f.id}>
        <strong class="font-display font-semibold tabular-nums {f.tone}">{formatBytes(f.bytes)}</strong>
        <span class="text-[13px] text-muted">{f.text}</span>
      </p>
    {/each}
  </div>
  {#if chips.length}
    <div role="group" aria-label={strings.results.categories} class="flex min-w-0 flex-[1_1_auto] flex-wrap justify-end gap-1.5">
      {#each chips as c (c.key)}
        {@const pressed = isActive(c.filter)}
        <button
          type="button"
          aria-pressed={pressed}
          onclick={() => toggleFilter(c.filter)}
          class="cursor-pointer rounded-full border px-2.5 py-1 text-xs whitespace-nowrap tabular-nums transition-colors {pressed
            ? 'border-accent bg-accent text-on-accent'
            : 'border-stroke bg-chip text-primary hover:bg-field'} {c.key === 'rule' ? 'font-mono' : ''}"
        >
          {c.filter.label} · {formatBytes(c.bytes)}
        </button>
      {/each}
    </div>
  {/if}
</section>
