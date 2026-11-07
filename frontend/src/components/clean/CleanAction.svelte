<script lang="ts">
  import CleanDialog from './CleanDialog.svelte'
  import { formatBytes } from '$lib/format'
  import { clean } from '$lib/stores/clean.svelte'
  import { selection, selectedBytes } from '$lib/stores/selection.svelte'
  import { strings } from '$lib/strings'

  let open = $state(false)
  let btn = $state<HTMLButtonElement>()

  function close() {
    open = false
    // The dialog restores focus too; this covers a button that was re-rendered meanwhile.
    queueMicrotask(() => btn?.focus())
  }
</script>

<button
  bind:this={btn}
  type="button"
  disabled={selection.paths.size === 0 || clean.phase === 'executing'}
  onclick={() => (open = true)}
  class="inline-flex cursor-pointer items-center justify-center rounded-sm border border-accent bg-accent px-3 py-1.5 text-[13px] font-semibold text-on-accent tabular-nums transition-colors enabled:hover:border-accent-hover enabled:hover:bg-accent-hover disabled:cursor-not-allowed disabled:opacity-45"
>
  {strings.clean.button(selection.paths.size, formatBytes(selectedBytes()))}
</button>
<CleanDialog {open} onClose={close} />
