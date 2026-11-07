<script lang="ts">
  import { tick, untrack } from 'svelte'
  import Icon from '../ui/Icon.svelte'
  import CleanReview from './CleanReview.svelte'
  import CleanProgress from './CleanProgress.svelte'
  import CleanSummary from './CleanSummary.svelte'
  import { clean, startClean, resetClean } from '$lib/stores/clean.svelte'
  import { selection } from '$lib/stores/selection.svelte'
  import { strings } from '$lib/strings'

  let { open, onClose }: { open: boolean; onClose: () => void } = $props()

  let dlg = $state<HTMLDialogElement>()
  let origin: HTMLElement | null = null

  $effect(() => {
    if (!dlg) return
    if (open && !dlg.open) {
      origin = document.activeElement as HTMLElement | null
      dlg.showModal()
      untrack(() => void startClean([...selection.paths.keys()]))
      void tick().then(() => dlg?.querySelector<HTMLElement>('input[type="radio"]:checked')?.focus())
    } else if (!open && dlg.open) {
      dlg.close()
    }
  })

  /** Esc: closes unless a cleaning is running. The native close is always prevented; the parent owns `open`. */
  function onCancel(e: Event) {
    e.preventDefault()
    if (clean.phase !== 'executing') onClose()
  }

  function onNativeClose() {
    resetClean()
    origin?.focus()
    if (open) onClose()
  }
</script>

<dialog
  bind:this={dlg}
  aria-labelledby="clean-title"
  oncancel={onCancel}
  onclose={onNativeClose}
  class="glass m-auto w-[min(40rem,calc(100vw-2rem))] rounded-lg bg-surface p-5 text-primary backdrop:bg-black/40"
>
  <div class="mb-3 flex items-center justify-between gap-3">
    <h2 id="clean-title" class="text-lg font-semibold text-primary">
      {clean.phase === 'done' ? strings.clean.doneTitle : strings.clean.title}
    </h2>
    {#if clean.phase === 'planning' || clean.phase === 'review'}
      <button
        type="button"
        class="cursor-pointer rounded-md p-1 text-primary hover:bg-black/5"
        aria-label={strings.clean.close}
        onclick={onClose}><Icon name="denied" size={18} /></button
      >
    {/if}
  </div>
  <div class="flex max-h-[70vh] flex-col gap-3 overflow-auto">
    {#if open}
      {#if clean.phase === 'executing'}
        <CleanProgress />
      {:else if clean.phase === 'done'}
        <CleanSummary {onClose} />
      {:else}
        <CleanReview {onClose} />
      {/if}
    {/if}
  </div>
</dialog>
