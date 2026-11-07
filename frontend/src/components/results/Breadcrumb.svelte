<script lang="ts">
  import type { NodeDTO } from '$lib/api'
  import { goUp } from '$lib/stores/results.svelte'
  import { strings } from '$lib/strings'

  let { volume, path }: { volume: string; path: NodeDTO[] } = $props()
</script>

<nav aria-label={strings.results.breadcrumb}>
  <ol class="flex flex-wrap items-center gap-1 text-sm">
    <li>
      <button
        type="button"
        class="cursor-pointer rounded-sm px-1 text-primary hover:bg-black/5"
        aria-current={path.length === 0 ? 'page' : undefined}
        onclick={() => goUp(0)}>{volume || strings.results.root}</button
      >
    </li>
    {#each path as n, i (n.id)}
      <li class="flex items-center gap-1">
        <span aria-hidden="true" class="text-muted">/</span>
        <button
          type="button"
          class="cursor-pointer rounded-sm px-1 text-primary hover:bg-black/5"
          aria-current={i === path.length - 1 ? 'page' : undefined}
          onclick={() => goUp(i + 1)}>{n.name}</button
        >
      </li>
    {/each}
  </ol>
</nav>
