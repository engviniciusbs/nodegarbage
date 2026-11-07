<script lang="ts">
  let { value, max = 100, label }: { value: number; max?: number; label: string } = $props()
  const pct = $derived(max > 0 ? Math.min(100, Math.max(0, (value / max) * 100)) : 0)
</script>

<div
  class="h-2 w-full overflow-hidden rounded-sm bg-black/10"
  role="progressbar"
  aria-label={label}
  aria-valuemin="0"
  aria-valuemax={max}
  aria-valuenow={value}
>
  <div class="h-full bg-accent transition-[width]" style:width="{pct}%"></div>
</div>
