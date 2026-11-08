<script lang="ts">
  import { strings } from '$lib/strings'

  let { lines = 3, label = strings.common.loading }: { lines?: number; label?: string } = $props()
</script>

<div class="space-y-3" role="status" aria-busy="true" aria-label={label}>
  {#each Array.from({ length: lines }, (_, i) => i) as i (i)}
    <div class="h-10 animate-pulse rounded-md bg-hover"></div>
  {/each}
</div>
