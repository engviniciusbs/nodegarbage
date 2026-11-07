<script lang="ts">
  import { untrack } from 'svelte'
  import SummaryBar from '../components/results/SummaryBar.svelte'
  import Breadcrumb from '../components/results/Breadcrumb.svelte'
  import TreeList from '../components/results/TreeList.svelte'
  import Treemap from '../components/treemap/Treemap.svelte'
  import DetailsPanel from '../components/results/DetailsPanel.svelte'
  import SelectionBar from '../components/results/SelectionBar.svelte'
  import CleanAction from '../components/clean/CleanAction.svelte'
  import Button from '../components/ui/Button.svelte'
  import EmptyState from '../components/ui/EmptyState.svelte'
  import ErrorState from '../components/ui/ErrorState.svelte'
  import Skeleton from '../components/ui/Skeleton.svelte'
  import { GetSummary } from '$lib/api'
  import type { SummaryDTO } from '$lib/api'
  import { route } from '$lib/route.svelte'
  import { results, loadVolume, openNode, refresh, toggleCategory } from '$lib/stores/results.svelte'
  import { clean } from '$lib/stores/clean.svelte'
  import { strings } from '$lib/strings'

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
    loadSummary(volume)
  })

  function loadSummary(volume: string) {
    if (!volume) return
    GetSummary(volume)
      .then((s) => {
        if (route.volume === volume) summary = s
      })
      .catch(() => {})
  }

  // A finished cleaning changes the totals: refetch the summary (the list is refreshed by the clean store).
  $effect(() => {
    if (clean.doneCount > 0) untrack(() => loadSummary(route.volume))
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
    <div class="grid min-h-0 flex-1 grid-cols-1 gap-3 lg:grid-cols-[minmax(0,1fr)_20rem] xl:grid-cols-[minmax(0,1fr)_minmax(0,1.2fr)_20rem]">
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
      <!-- lg: treemap stacks above details in the right column; xl: wrapper dissolves into 3 columns. -->
      <div class="flex min-h-0 min-w-0 flex-col gap-3 xl:contents">
        <div class="h-56 min-w-0 shrink-0 overflow-hidden xl:h-full xl:min-h-0" aria-hidden="true">
          {#if items.length > 0 && !results.error}
            <Treemap
              nodes={items}
              {focusedId}
              onOpen={(id) => void openNode(id)}
              onSelect={(id) => (results.focusRequest = id)}
              onHover={(id) => (results.hoverId = id)}
            />
          {/if}
        </div>
        <DetailsPanel node={focused} />
      </div>
    </div>
    <SelectionBar>
      {#snippet actions()}<CleanAction />{/snippet}
    </SelectionBar>
  </div>
{/if}
