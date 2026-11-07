<script lang="ts">
  import Panel from '../ui/Panel.svelte'
  import type { ProgressDTO, ScanDoneDTO } from '$lib/api'
  import { formatBytes, formatCount } from '$lib/format'
  import { strings } from '$lib/strings'

  let {
    root,
    progress,
    done,
    active,
  }: { root: string; progress?: ProgressDTO; done?: ScanDoneDTO; active: boolean } = $props()

  const status = $derived(
    done
      ? done.error === 'canceled'
        ? strings.scan.canceled
        : done.error
          ? strings.scan.failed
          : strings.scan.done
      : active
        ? strings.scan.running
        : strings.scan.waiting,
  )
</script>

<Panel class="space-y-2">
  <div class="flex items-baseline justify-between gap-2">
    <span class="font-semibold text-primary">{root}</span>
    <span class="text-sm text-muted">{status}</span>
  </div>
  {#if progress}
    <dl class="grid grid-cols-2 gap-x-4 gap-y-1 text-sm sm:grid-cols-4">
      <div><dd class="font-medium text-primary">{formatCount(progress.files)}</dd><dt class="text-xs text-muted">{strings.scan.files}</dt></div>
      <div><dd class="font-medium text-primary">{formatCount(progress.dirs)}</dd><dt class="text-xs text-muted">{strings.scan.dirs}</dt></div>
      <div><dd class="font-medium text-primary">{formatBytes(progress.bytes)}</dd><dt class="text-xs text-muted">{strings.scan.bytes}</dt></div>
      <div><dd class="font-medium text-primary">{strings.scan.perSec(formatBytes(progress.bytesPerSec))}</dd><dt class="text-xs text-muted">{strings.scan.denied}: {formatCount(progress.denied)}</dt></div>
    </dl>
    {#if active && !done}
      <p class="truncate font-mono text-xs text-muted" title={progress.current}>{progress.current}</p>
    {/if}
  {/if}
</Panel>
