<script lang="ts">
  import Icon from '../ui/Icon.svelte'
  import { formatBytes } from '$lib/format'
  import { clean } from '$lib/stores/clean.svelte'
  import { refusalText } from '$lib/reasons'
  import { strings } from '$lib/strings'

  let { onClose }: { onClose: () => void } = $props()

  const results = $derived(clean.summary?.results ?? [])
  const count = (s: string) => results.filter((r) => r.status === s).length
  /** Skipped, failed, or partially removed items. */
  const problems = $derived(results.filter((r) => r.status !== 'ok' || r.reason === 'partial'))
  const label = (r: { status: string }) =>
    r.status === 'error' ? strings.clean.statusError : r.status === 'ok' ? strings.clean.statusPartial : strings.clean.statusSkipped

  let closeBtn = $state<HTMLButtonElement>()
  $effect(() => closeBtn?.focus())
</script>

<div class="flex flex-col gap-3">
  <div>
    <p class="text-xs text-muted">{strings.clean.freed}</p>
    <p class="font-display text-2xl font-semibold tabular-nums text-primary">{formatBytes(clean.summary?.freed ?? 0)}</p>
  </div>
  <ul class="flex flex-wrap gap-4 text-sm text-primary">
    <li>{strings.clean.okCount(count('ok'))}</li>
    <li>{strings.clean.skippedCount(count('skipped'))}</li>
    <li>{strings.clean.errorCount(count('error'))}</li>
  </ul>
  {#if clean.summary?.warning === 'snapshot_not_saved'}
    <p class="text-sm text-muted">{strings.clean.snapshotNotSaved}</p>
  {/if}
  {#if problems.length > 0}
    <section class="flex flex-col gap-2">
      <h3 class="text-sm font-semibold text-primary">{strings.clean.problemsTitle}</h3>
      <ul class="flex max-h-48 flex-col gap-1 overflow-auto">
        {#each problems as r, i (r.path + i)}
          <li class="rounded-sm bg-well px-3 py-2 text-sm">
            <span class="flex items-center gap-2 text-primary">
              <span class="text-warn-text"><Icon name={r.status === 'error' ? 'denied' : 'caution'} size={14} /></span>
              <span class="font-medium">{label(r)}</span>
            </span>
            <span class="break-all font-mono text-xs text-primary">{r.path}</span>
            <p class="text-xs text-muted">{refusalText(r.reason)}</p>
          </li>
        {/each}
      </ul>
    </section>
  {/if}
  <div class="flex justify-end">
    <button
      bind:this={closeBtn}
      type="button"
      class="inline-flex cursor-pointer items-center rounded-sm border border-accent bg-accent px-3 py-1.5 text-[13px] font-semibold text-on-accent hover:border-accent-hover hover:bg-accent-hover"
      onclick={onClose}>{strings.clean.close}</button
    >
  </div>
</div>
