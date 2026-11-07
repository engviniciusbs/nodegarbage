<script lang="ts">
  import VolumeProgress from '../components/scan/VolumeProgress.svelte'
  import Button from '../components/ui/Button.svelte'
  import EmptyState from '../components/ui/EmptyState.svelte'
  import ErrorState from '../components/ui/ErrorState.svelte'
  import { scanState, cancelScan, CANCELED } from '$lib/stores/scan.svelte'
  import { route } from '$lib/route.svelte'
  import { strings } from '$lib/strings'

  const failures = $derived(
    scanState.queue.map((r) => scanState.done[r]).filter((d) => d && d.error && d.error !== CANCELED),
  )
  const canceled = $derived(scanState.queue.some((r) => scanState.done[r]?.error === CANCELED))
  const activeRoot = $derived(scanState.queue.find((r) => !scanState.done[r]))
  const back = () => (route.current = 'home')
</script>

{#if scanState.queue.length === 0}
  <EmptyState title={strings.scan.idle}>
    {#snippet action()}<Button onclick={back}>{strings.scan.back}</Button>{/snippet}
  </EmptyState>
{:else}
  <div class="space-y-3">
    <!-- Polite live region: updated once per progress event (backend throttles to <=10/s). -->
    <div aria-live="polite" aria-label={strings.scan.queueLabel} class="space-y-3">
      {#each scanState.queue as root (root)}
        <VolumeProgress
          {root}
          progress={scanState.progress[root]}
          done={scanState.done[root]}
          active={root === activeRoot && scanState.running}
        />
      {/each}
    </div>

    {#each failures as f (f.volume)}
      <ErrorState title={strings.scan.errorTitle} message={`${f.volume} — ${f.error}`} />
    {/each}
    {#if canceled}
      <p class="text-sm text-muted">{strings.scan.canceledBody}</p>
    {/if}

    <div class="flex justify-end gap-3">
      {#if scanState.running}
        <Button onclick={cancelScan}>{strings.scan.cancel}</Button>
      {:else}
        <Button variant="primary" onclick={back}>{strings.scan.back}</Button>
      {/if}
    </div>
  </div>
{/if}
