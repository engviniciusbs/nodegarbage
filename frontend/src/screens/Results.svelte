<script lang="ts">
  import { untrack } from 'svelte'
  import ResultsHeader from '../components/results/ResultsHeader.svelte'
  import SummaryBar from '../components/results/SummaryBar.svelte'
  import Breadcrumb from '../components/results/Breadcrumb.svelte'
  import ViewTabs, { type ResultsView } from '../components/results/ViewTabs.svelte'
  import ListHeader from '../components/results/ListHeader.svelte'
  import TreeList from '../components/results/TreeList.svelte'
  import Treemap from '../components/treemap/Treemap.svelte'
  import TreemapLegend from '../components/treemap/TreemapLegend.svelte'
  import DetailsPanel from '../components/results/DetailsPanel.svelte'
  import SelectionBar from '../components/results/SelectionBar.svelte'
  import CleanAction from '../components/clean/CleanAction.svelte'
  import Button from '../components/ui/Button.svelte'
  import EmptyState from '../components/ui/EmptyState.svelte'
  import ErrorState from '../components/ui/ErrorState.svelte'
  import Skeleton from '../components/ui/Skeleton.svelte'
  import { GetSummary, ListVolumes } from '$lib/api'
  import type { SummaryDTO } from '$lib/api'
  import { route } from '$lib/route.svelte'
  import { results, loadVolume, openNode, refresh, toggleCategory } from '$lib/stores/results.svelte'
  import { clean } from '$lib/stores/clean.svelte'
  import { pickScannedVolume } from '$lib/volumes'
  import { strings } from '$lib/strings'

  const PANEL_ID = 'results-view'

  let summary = $state<SummaryDTO | undefined>()
  let focusedId = $state<number | null>(null)
  let view = $state<ResultsView>('list')
  /** True while looking for a scanned volume to open (Results reached from the sidebar with no volume). */
  let picking = $state(false)

  // No volume chosen yet: open the most recently scanned one. The empty state is only for 'nothing scanned'.
  $effect(() => {
    if (route.volume) return
    picking = true
    Promise.resolve()
      .then(() => ListVolumes())
      .then((vols) => {
        const root = pickScannedVolume(vols ?? [])
        if (root && !route.volume) route.volume = root
      })
      .catch(() => {})
      .finally(() => (picking = false))
  })

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

  // Treemap leaf click: in the map tab there is no visible list, so focus the item for the details panel directly.
  function selectFromMap(id: number) {
    focusedId = id
    results.focusRequest = id
  }
</script>

<!-- The page never scrolls here: only the list (or map) and the details panel scroll inside. -->
<div class="flex h-full min-h-0 flex-col">
  <ResultsHeader volume={route.volume} />
  {#if !route.volume}
    {#if picking}
      <div class="px-6"><Skeleton lines={4} /></div>
    {:else}
      <EmptyState title={strings.results.emptyVolume} />
    {/if}
  {:else}
    {#if summary}<SummaryBar {summary} />{/if}
    <div class="flex shrink-0 items-center justify-between gap-3 px-6 pt-3.5 pb-2">
      <Breadcrumb volume={results.volume} path={results.path} />
      <ViewTabs bind:view controls={PANEL_ID} />
    </div>
    <div
      class="grid min-h-0 flex-1 grid-cols-[minmax(0,1fr)_300px] gap-4 px-6 pb-3 min-[1200px]:grid-cols-[minmax(0,1fr)_360px]"
    >
      <div
        id={PANEL_ID}
        role="tabpanel"
        aria-labelledby="results-tab-{view}"
        class="flex min-h-0 min-w-0 flex-col overflow-hidden rounded-md border border-stroke bg-card"
      >
        {#if results.error}
          <ErrorState message={results.error} onretry={refresh} />
        {:else if results.loading && results.children.length === 0}
          <div class="p-3"><Skeleton lines={8} /></div>
        {:else if items.length === 0}
          <EmptyState title={results.categoryFilter ? strings.results.emptyFiltered : strings.results.emptyFolder}>
            {#snippet action()}
              {#if results.categoryFilter}
                <Button onclick={() => toggleCategory(results.categoryFilter!)}>{strings.results.clearFilter}</Button>
              {/if}
            {/snippet}
          </EmptyState>
        {:else}
          <!-- The list stays mounted (hidden) in the map tab: focus, scroll and focus requests survive the switch. -->
          <div class="list-panel flex min-h-0 flex-1 flex-col" hidden={view !== 'list'} data-view="list">
            <ListHeader />
            {#key `${results.currentId}|${results.categoryFilter}`}
              <TreeList {items} bind:focusedId />
            {/key}
          </div>
          {#if view === 'map'}
            <div class="flex min-h-0 flex-1 flex-col" data-view="map">
              <div class="relative m-2.5 min-h-0 flex-1">
                <Treemap
                  nodes={items}
                  {focusedId}
                  onOpen={(id) => void openNode(id)}
                  onSelect={selectFromMap}
                  onHover={(id) => (results.hoverId = id)}
                />
              </div>
              <TreemapLegend />
            </div>
          {/if}
        {/if}
      </div>
      <DetailsPanel node={focused} />
    </div>
    <SelectionBar>
      {#snippet actions()}<CleanAction />{/snippet}
    </SelectionBar>
  {/if}
</div>
