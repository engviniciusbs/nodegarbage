<script lang="ts">
  import { onDestroy } from 'svelte'
  import Button from '../ui/Button.svelte'
  import { strings } from '$lib/strings'

  let { command }: { command: string } = $props()

  let state = $state<'idle' | 'copied' | 'error'>('idle')
  let timer: ReturnType<typeof setTimeout> | undefined

  async function copy() {
    try {
      await navigator.clipboard.writeText(command)
      state = 'copied'
    } catch {
      state = 'error'
    }
    clearTimeout(timer)
    timer = setTimeout(() => (state = 'idle'), 2000)
  }
  onDestroy(() => clearTimeout(timer))
</script>

<div class="flex flex-col gap-1">
  <div class="flex items-center gap-2 rounded-sm border border-stroke bg-field py-1.5 pr-1.5 pl-2.5">
    <code class="min-w-0 flex-1 overflow-x-auto font-mono text-xs whitespace-nowrap select-text">{command}</code>
    <Button class="shrink-0" onclick={copy}>{strings.results.copy}</Button>
  </div>
  <span class="text-xs text-muted" role="status">
    {#if state === 'copied'}{strings.results.copied}{:else if state === 'error'}{strings.results.copyError}{/if}
  </span>
</div>
