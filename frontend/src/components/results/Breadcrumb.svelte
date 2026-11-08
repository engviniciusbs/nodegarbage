<script lang="ts">
  import type { NodeDTO } from '$lib/api'
  import { goUp } from '$lib/stores/results.svelte'
  import { strings } from '$lib/strings'

  let { volume, path }: { volume: string; path: NodeDTO[] } = $props()

  const crumb =
    'cursor-pointer rounded-[4px] px-1.5 py-[3px] text-muted hover:bg-hover hover:text-primary aria-[current=page]:font-semibold aria-[current=page]:text-primary'
</script>

<nav aria-label={strings.results.breadcrumb} class="min-w-0">
  <ol class="flex min-w-0 flex-wrap items-center gap-1 text-[13px]">
    <li>
      <button
        type="button"
        class={crumb}
        aria-current={path.length === 0 ? 'page' : undefined}
        onclick={() => goUp(0)}>{volume || strings.results.root}</button
      >
    </li>
    {#each path as n, i (n.id)}
      <li class="flex min-w-0 items-center gap-1">
        <span aria-hidden="true" class="text-muted opacity-60">/</span>
        <button
          type="button"
          class="{crumb} max-w-[16rem] truncate"
          aria-current={i === path.length - 1 ? 'page' : undefined}
          onclick={() => goUp(i + 1)}>{n.name}</button
        >
      </li>
    {/each}
  </ol>
</nav>
