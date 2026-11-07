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

<div class="flex items-start gap-2">
  <code class="min-w-0 flex-1 rounded-sm bg-black/5 px-2 py-1 font-mono text-xs break-all select-text">{command}</code>
  <Button variant="ghost" class="shrink-0 !px-2 !py-1 text-xs" onclick={copy}>{strings.results.copy}</Button>
  <span class="text-xs text-muted" role="status">
    {#if state === 'copied'}{strings.results.copied}{:else if state === 'error'}{strings.results.copyError}{/if}
  </span>
</div>
