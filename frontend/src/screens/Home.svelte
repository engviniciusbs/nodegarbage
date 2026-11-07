<script lang="ts">
  import { onMount } from 'svelte'
  import VolumeCard from '../components/home/VolumeCard.svelte'
  import Button from '../components/ui/Button.svelte'
  import Skeleton from '../components/ui/Skeleton.svelte'
  import EmptyState from '../components/ui/EmptyState.svelte'
  import ErrorState from '../components/ui/ErrorState.svelte'
  import { ListVolumes } from '$lib/api'
  import type { VolumeDTO } from '$lib/api'
  import { route } from '$lib/route.svelte'
  import { startScan } from '$lib/stores/scan.svelte'
  import { strings } from '$lib/strings'

  let volumes = $state<VolumeDTO[]>([])
  let loading = $state(true)
  let error = $state('')
  let selected = $state<string[]>([])
  let startError = $state('')

  async function load() {
    loading = true
    error = ''
    try {
      volumes = await ListVolumes()
    } catch (e) {
      error = `${strings.home.loadError} ${e instanceof Error ? e.message : String(e)}`
    } finally {
      loading = false
    }
  }

  function toggle(root: string) {
    selected = selected.includes(root) ? selected.filter((r) => r !== root) : [...selected, root]
  }

  async function analyze() {
    startError = ''
    try {
      await startScan(selected)
      route.current = 'scan'
    } catch (e) {
      startError = `${strings.home.startError} ${e instanceof Error ? e.message : String(e)}`
    }
  }

  onMount(load)
</script>

<div class="max-w-[960px] space-y-4">
  {#if loading}
    <Skeleton lines={2} />
  {:else if error}
    <ErrorState message={error} onretry={load} />
  {:else if volumes.length === 0}
    <EmptyState title={strings.home.emptyTitle} description={strings.home.emptyDescription} />
  {:else}
    <p class="text-sm text-muted">{strings.home.subtitle}</p>
    <ul class="space-y-2" aria-label={strings.home.listLabel}>
      {#each volumes as v (v.root)}
        <li>
          <VolumeCard volume={v} checked={selected.includes(v.root)} onToggle={() => toggle(v.root)} />
        </li>
      {/each}
    </ul>
    {#if startError}<p role="alert" class="text-sm text-warn-text">{startError}</p>{/if}
    <div class="flex items-center justify-end gap-3">
      <span class="text-sm text-muted" aria-live="polite">{strings.home.selectedCount(selected.length)}</span>
      <Button variant="primary" disabled={selected.length === 0} onclick={analyze}>{strings.home.analyze}</Button>
    </div>
  {/if}
</div>
