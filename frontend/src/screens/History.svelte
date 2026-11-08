<script lang="ts">
  import { onMount } from 'svelte'
  import Icon, { type IconName } from '../components/ui/Icon.svelte'
  import EmptyState from '../components/ui/EmptyState.svelte'
  import ErrorState from '../components/ui/ErrorState.svelte'
  import Skeleton from '../components/ui/Skeleton.svelte'
  import { GetHistory } from '$lib/api'
  import type { HistoryDTO } from '$lib/api'
  import { formatBytes, formatDate } from '$lib/format'
  import { refusalText } from '$lib/reasons'
  import { strings } from '$lib/strings'

  let data = $state<HistoryDTO | null>(null)
  let loading = $state(true)
  let error = $state('')

  async function load() {
    loading = true
    error = ''
    try {
      data = await GetHistory(200)
    } catch (e) {
      error = `${strings.history.loadError} ${typeof e === 'string' ? e : e instanceof Error ? e.message : String(e)}`
    } finally {
      loading = false
    }
  }
  onMount(() => void load())

  const statusIcon: Record<string, IconName> = { ok: 'safe', skipped: 'caution', error: 'denied' }
  const statusText = (s: string) =>
    s === 'ok' ? strings.history.ok : s === 'skipped' ? strings.history.skipped : s === 'error' ? strings.history.error : s
  const modeText = (m: string) => (m === 'permanent' ? strings.history.permanent : strings.history.recycle)
  const dateText = (at: unknown) => {
    const t = Date.parse(String(at))
    return Number.isNaN(t) ? '' : formatDate(t / 1000)
  }
</script>

{#if loading}
  <Skeleton lines={6} />
{:else if error}
  <ErrorState message={error} onretry={load} />
{:else if !data || data.items.length === 0}
  <EmptyState title={strings.history.emptyTitle} description={strings.history.emptyBody} icon="history" />
{:else}
  <div class="flex flex-col gap-4">
    <div class="rounded-md border border-stroke bg-card p-4">
      <p class="text-xs text-muted">{strings.history.totalFreed}</p>
      <p class="font-display text-2xl font-semibold tabular-nums text-primary">{formatBytes(data.totalFreed)}</p>
    </div>
    <div class="overflow-auto rounded-md border border-stroke bg-card">
      <table class="w-full text-left text-sm text-primary" aria-label={strings.history.tableLabel}>
        <thead class="text-[11px] tracking-[.04em] text-muted uppercase">
          <tr>
            <th scope="col" class="px-3 py-2 font-medium">{strings.history.path}</th>
            <th scope="col" class="px-3 py-2 font-medium">{strings.history.size}</th>
            <th scope="col" class="px-3 py-2 font-medium">{strings.history.mode}</th>
            <th scope="col" class="px-3 py-2 font-medium">{strings.history.date}</th>
            <th scope="col" class="px-3 py-2 font-medium">{strings.history.status}</th>
          </tr>
        </thead>
        <tbody>
          {#each data.items as it, i (it.path + i)}
            <tr class="border-t border-stroke-soft align-top">
              <td class="break-all px-3 py-2 font-mono text-xs">{it.path}</td>
              <td class="px-3 py-2 tabular-nums">{formatBytes(it.size)}</td>
              <td class="px-3 py-2">{modeText(it.mode)}</td>
              <td class="px-3 py-2 tabular-nums">{dateText(it.at)}</td>
              <td class="px-3 py-2">
                <span class="inline-flex items-center gap-1.5">
                  <span class={it.status === 'ok' ? 'text-safe' : 'text-warn-text'}>
                    <Icon name={statusIcon[it.status] ?? 'unknown'} size={14} />
                  </span>
                  <span>{statusText(it.status)}</span>
                </span>
                {#if it.reason}<p class="text-xs text-muted">{refusalText(it.reason)}</p>{/if}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  </div>
{/if}
