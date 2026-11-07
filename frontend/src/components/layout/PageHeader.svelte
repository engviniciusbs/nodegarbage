<script lang="ts">
  import type { Snippet } from 'svelte'

  let { title, meta }: { title: string; meta?: Snippet } = $props()
</script>

<header class="flex shrink-0 flex-wrap items-baseline justify-between gap-x-4 gap-y-1 px-6 pt-5 pb-3">
  <h1 class="m-0 font-display text-2xl font-semibold text-primary">{title}</h1>
  {#if meta}<div class="text-xs text-muted">{@render meta()}</div>{/if}
</header>
