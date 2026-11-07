<script lang="ts">
  import Icon from './Icon.svelte'
  import Button from './Button.svelte'
  import { strings } from '$lib/strings'

  let {
    message,
    title = strings.common.error,
    onretry,
  }: { message: string; title?: string; onretry?: () => void } = $props()
</script>

<div role="alert" class="flex flex-col items-center gap-2 py-12 text-center">
  <span class="text-warn-text"><Icon name="caution" size={40} /></span>
  <h2 class="text-base font-semibold text-primary">{title}</h2>
  <p class="max-w-md text-sm text-muted">{message}</p>
  {#if onretry}
    <div class="mt-2"><Button variant="primary" onclick={onretry}>{strings.common.retry}</Button></div>
  {/if}
</div>
