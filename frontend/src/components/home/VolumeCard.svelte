<script lang="ts">
  import Panel from '../ui/Panel.svelte'
  import ProgressBar from '../ui/ProgressBar.svelte'
  import type { VolumeDTO } from '$lib/api'
  import { formatBytes, formatDate } from '$lib/format'
  import { strings } from '$lib/strings'

  let { volume, checked, onToggle }: { volume: VolumeDTO; checked: boolean; onToggle: () => void } = $props()

  const name = $derived(volume.label || volume.root)
  const usedPct = $derived(volume.total > 0 ? Math.round(((volume.total - volume.free) / volume.total) * 100) : 0)
  const lastScanSec = $derived(volume.lastScan ? Date.parse(volume.lastScan) / 1000 : NaN)
  const hasScan = $derived(Number.isFinite(lastScanSec) && new Date(lastScanSec * 1000).getFullYear() > 2000)
</script>

<Panel class="relative">
  <label class="flex cursor-pointer items-start gap-3">
    <input type="checkbox" class="mt-1 size-4 accent-accent" {checked} onchange={onToggle} />
    <span class="min-w-0 flex-1 space-y-2">
      <span class="flex items-baseline justify-between gap-2">
        <span class="truncate text-base font-semibold text-primary">{name}</span>
        <span class="shrink-0 text-xs text-muted">{volume.root} · {volume.fs}</span>
      </span>
      <ProgressBar
        value={usedPct}
        label={strings.home.used(name)}
        overlay={volume.total > 0 ? (volume.estimatedJunk / volume.total) * 100 : 0}
      />
      <span class="flex flex-wrap justify-between gap-x-3 text-sm text-muted">
        <span>{strings.home.free(formatBytes(volume.free), formatBytes(volume.total))}</span>
        {#if volume.estimatedJunk > 0}
          <span class="font-medium text-warn-text">{strings.home.junk(formatBytes(volume.estimatedJunk))}</span>
        {/if}
      </span>
      <span class="block text-xs text-muted">
        {hasScan ? strings.home.lastScan(formatDate(lastScanSec)) : strings.home.neverScanned}
      </span>
    </span>
  </label>
</Panel>
