<script lang="ts">
  import Button from '../ui/Button.svelte'
  import ProgressBar from '../ui/ProgressBar.svelte'
  import { clean, cancelRun } from '$lib/stores/clean.svelte'
  import { strings } from '$lib/strings'

  let root = $state<HTMLElement>()
  $effect(() => root?.querySelector('button')?.focus())
</script>

<div bind:this={root} class="flex flex-col gap-3" aria-live="polite">
  <p class="text-sm font-medium text-primary">{strings.clean.running}</p>
  <ProgressBar value={clean.progress.done} max={clean.progress.total} label={strings.clean.progressLabel} />
  <p class="text-xs tabular-nums text-muted">{strings.clean.progress(clean.progress.done, clean.progress.total)}</p>
  {#if clean.progress.current}
    <p class="break-all font-mono text-xs text-primary">{clean.progress.current}</p>
  {/if}
  <div class="flex justify-end">
    <Button disabled={clean.canceling} onclick={cancelRun}>
      {clean.canceling ? strings.clean.cancelling : strings.clean.cancelRun}
    </Button>
  </div>
</div>
