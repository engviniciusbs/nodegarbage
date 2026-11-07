<script lang="ts">
  import { strings } from '$lib/strings'
</script>

<!-- Column captions for the tree below (same grid as TreeRow). Visual only: each row names itself. -->
<div
  aria-hidden="true"
  class="list-grid h-[34px] shrink-0 border-b border-stroke-soft text-[11px] tracking-[.04em] text-muted uppercase"
>
  <span></span>
  <span>{strings.results.colName}</span>
  <span class="text-right">{strings.results.colSize}</span>
  <span class="col-files text-right">{strings.results.colFiles}</span>
  <span>{strings.results.colVerdict}</span>
</div>
