<script lang="ts">
  import { strings, type Verdict } from '$lib/strings'

  const swatch: Record<Verdict, string> = {
    safe: 'bg-tile-safe',
    caution: 'bg-tile-caution',
    keep: 'bg-tile-keep',
    unknown: 'bg-tile-unknown',
  }
  const order: Verdict[] = ['safe', 'caution', 'keep', 'unknown']
</script>

<ul aria-label={strings.results.legendLabel} class="m-0 flex shrink-0 list-none flex-wrap gap-3.5 px-3 pb-2.5 text-xs text-muted">
  {#each order as v (v)}
    <li class="flex items-center gap-1.5">
      <i aria-hidden="true" class="inline-block size-2.5 rounded-[2px] {swatch[v]}"></i>{strings.verdict[v]}
    </li>
  {/each}
</ul>
