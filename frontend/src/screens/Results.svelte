<script lang="ts">
  import { untrack, type Snippet } from 'svelte'
  import SummaryBar from '../components/results/SummaryBar.svelte'
  import Breadcrumb from '../components/results/Breadcrumb.svelte'
  import TreeList from '../components/results/TreeList.svelte'
  import DetailsPanel from '../components/results/DetailsPanel.svelte'
  import SelectionBar from '../components/results/SelectionBar.svelte'
  import Button from '../components/ui/Button.svelte'
  import EmptyState from '../components/ui/EmptyState.svelte'
  import ErrorState from '../components/ui/ErrorState.svelte'
  import Skeleton from '../components/ui/Skeleton.svelte'
  import { GetSummary } from '$lib/api'
  import type { SummaryDTO } from '$lib/api'
  import { route } from '$lib/route.svelte'
  import { results, loadVolume, refresh, toggleCategory } from '$lib/stores/results.svelte'
  import { strings } from '$lib/strings'

  /** Task 21 plugs the "Limpar" button into the selection bar. */
  let { barActions }: { barActions?: Snippet } = $props()

  let summary = $state<SummaryDTO | undefined>()
  let focusedId = $state<number | null>(null)

  $effect(() => {
    const volume = route.volume
    summary = undefined
    focusedId = null
    // Store calls read their own state; keep that out of this effect's dependencies.
    untrack(() => {
      if (volume !== results.volume) void loadVolume(volume)
      else if (volume) void refresh()
    })
    if (!volume) return
    GetSummary(volume)
      .then((s) => {
        if (route.volume === volume) summary = s
      })
      .catch(() => {})
  })

  const items = $derived(
    results.categoryFilter ? results.children.filter((c) => c.category === results.categoryFilter) : results.children,
  )
  const focused = $derived(items.find((n) => n.id === focusedId) ?? items[0])
</script>

{#if !route.volume}
  <EmptyState title={strings.results.emptyVolume} />
{:else}
  <div class="flex h-full min-h-0 flex-col gap-3">
    {#if summary}<SummaryBar {summary} />{/if}
    <Breadcrumb volume={results.volume} path={results.path} />
    <div class="grid min-h-0 flex-1 grid-cols-1 gap-3 lg:grid-cols-[1fr_20rem]">
      <div class="min-h-64 min-w-0">
        {#if results.error}
          <ErrorState message={results.error} onretry={refresh} />
        {:else if results.loading && results.children.length === 0}
          <Skeleton lines={8} />
        {:else if items.length === 0}
          <EmptyState
            title={results.categoryFilter ? strings.results.emptyFiltered : strings.results.emptyFolder}
          >
            {#snippet action()}
              {#if results.categoryFilter}
                <Button onclick={() => toggleCategory(results.categoryFilter!)}>{strings.results.clearFilter}</Button>
              {/if}
            {/snippet}
          </EmptyState>
        {:else}
          {#key `${results.currentId}|${results.categoryFilter}`}
            <TreeList {items} bind:focusedId />
          {/key}
        {/if}
      </div>
      <DetailsPanel node={focused} />
    </div>
    <SelectionBar actions={barActions} />
  </div>
{/if}
