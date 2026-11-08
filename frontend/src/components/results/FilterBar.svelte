<script lang="ts">
  import Icon from '../ui/Icon.svelte'
  import type { FilteredDTO } from '$lib/api'
  import { formatBytes } from '$lib/format'
  import type { ActiveFilter } from '$lib/stores/results.svelte'
  import { strings } from '$lib/strings'

  let {
    filter,
    filtered,
    notice = '',
    onclear,
  }: { filter: ActiveFilter; filtered: FilteredDTO | null; notice?: string; onclear: () => void } = $props()
</script>

<!-- Takes the breadcrumb's place while a volume-wide filter is active. -->
<div role="region" aria-label={strings.results.filterRegion} class="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-[13px]">
  <p class="m-0 min-w-0 truncate font-semibold text-primary tabular-nums" aria-live="polite">
    {#if filtered}
      {strings.results.filterSummary(filter.label, filtered.count, formatBytes(filtered.totalBytes))}
    {:else}
      {strings.results.filterSummary(filter.label, 0, '…')}
    {/if}
  </p>
  {#if filtered?.truncated}
    <span aria-hidden="true" class="text-muted">·</span>
    <span class="text-xs text-muted">{strings.results.truncated}</span>
  {/if}
  <button
    type="button"
    aria-label={strings.results.clearFilter}
    title={strings.results.clearFilter}
    class="inline-flex size-6 cursor-pointer items-center justify-center rounded-sm text-muted hover:bg-hover hover:text-primary"
    onclick={onclear}><Icon name="close" size={14} /></button
  >
  {#if notice}
    <!-- Non-blocking: the filtered list below stays as it was. -->
    <p role="alert" class="m-0 basis-full text-xs text-warn-text">{notice}</p>
  {/if}
</div>
