<script lang="ts">
  import PageHeader from '../layout/PageHeader.svelte'
  import { ListVolumes } from '$lib/api'
  import { formatDate, formatDuration, formatTime } from '$lib/format'
  import { route } from '$lib/route.svelte'
  import { scanState, startScan } from '$lib/stores/scan.svelte'
  import { scanTime } from '$lib/volumes'
  import { strings } from '$lib/strings'

  let { volume }: { volume: string } = $props()

  let lastScanSec = $state<number | null>(null)
  let error = $state('')

  // When the volume was scanned comes from ListVolumes; how long it took is only known for a scan of this session.
  $effect(() => {
    const v = volume
    lastScanSec = null
    error = ''
    if (!v) return
    Promise.resolve()
      .then(() => ListVolumes())
      .then((vols) => {
        const found = vols?.find((x) => x.root === v)
        const t = found ? scanTime(found) : null
        if (volume === v && t !== null) lastScanSec = t / 1000
      })
      .catch(() => {})
  })

  const duration = $derived(scanState.done[volume]?.error ? 0 : (scanState.done[volume]?.durationMs ?? 0))

  async function rescan() {
    error = ''
    try {
      await startScan([volume])
      route.current = 'scan'
    } catch (e) {
      error = `${strings.results.rescanError} ${e instanceof Error ? e.message : String(e)}`
    }
  }
</script>

<PageHeader title={strings.nav.results}>
  {#snippet meta()}
    {#if volume}
      <span>{strings.results.metaVolume(volume)}</span>
      {#if lastScanSec !== null}
        <span> · {strings.results.metaScanned(formatDate(lastScanSec), formatTime(lastScanSec))}</span>
        {#if duration > 0}<span> {strings.results.metaDuration(formatDuration(duration))}</span>{/if}
      {/if}
      <button
        type="button"
        class="ml-2 cursor-pointer rounded-sm text-accent hover:text-accent-hover hover:underline"
        onclick={rescan}>{strings.results.rescan}</button
      >
      {#if error}<span role="alert" class="ml-2 text-warn-text">{error}</span>{/if}
    {/if}
  {/snippet}
</PageHeader>
