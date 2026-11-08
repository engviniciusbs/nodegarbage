<script lang="ts">
  let {
    value,
    max = 100,
    label,
    overlay = 0,
  }: {
    value: number
    max?: number
    label: string
    /** Decorative extra segment (same unit as value) drawn inside the filled part. Never the only carrier of information. */
    overlay?: number
  } = $props()
  const pct = $derived(max > 0 ? Math.min(100, Math.max(0, (value / max) * 100)) : 0)
  const overlayPct = $derived(max > 0 ? Math.min(pct, Math.max(0, (overlay / max) * 100)) : 0)
</script>

<div
  class="relative h-2 w-full overflow-hidden rounded-sm bg-stroke"
  role="progressbar"
  aria-label={label}
  aria-valuemin="0"
  aria-valuemax={max}
  aria-valuenow={value}
>
  <div class="h-full bg-accent transition-[width]" style:width="{pct}%"></div>
  {#if overlayPct > 0}
    <div class="absolute inset-y-0 bg-warn" style:width="{overlayPct}%" style:left="{pct - overlayPct}%" aria-hidden="true"></div>
  {/if}
</div>
