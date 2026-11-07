<script lang="ts">
  import CleanDialog from './CleanDialog.svelte'
  import { selection } from '$lib/stores/selection.svelte'
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
  disabled={selection.paths.size === 0}
  onclick={() => (open = true)}
  class="inline-flex cursor-pointer items-center justify-center rounded-md bg-accent px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-accent-hover disabled:pointer-events-none disabled:opacity-50"
>
  {strings.clean.button(selection.paths.size)}
</button>
<CleanDialog {open} onClose={close} />
