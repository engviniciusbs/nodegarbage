<script lang="ts">
  import type { Snippet } from 'svelte'
  import Icon, { type IconName } from './Icon.svelte'

  let {
    title,
    description,
    icon = 'inbox',
    action,
  }: { title: string; description?: string; icon?: IconName; action?: Snippet } = $props()
</script>

<div role="status" class="flex flex-col items-center gap-2 py-12 text-center">
  <span class="text-muted"><Icon name={icon} size={40} /></span>
  <h2 class="text-base font-semibold text-primary">{title}</h2>
  {#if description}<p class="max-w-md text-sm text-muted">{description}</p>{/if}
  {#if action}<div class="mt-2">{@render action()}</div>{/if}
</div>
