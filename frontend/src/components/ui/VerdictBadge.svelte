<script lang="ts">
  import Icon from './Icon.svelte'
  import { strings, type Verdict } from '$lib/strings'

  let { verdict, source }: { verdict: Verdict; source?: string } = $props()

  // Colour tints the chip and icon; the label stays --text-primary (contrast).
  const tone: Record<Verdict, string> = {
    safe: 'text-safe bg-safe/10',
    caution: 'text-warn-text bg-warn/20',
    keep: 'text-keep bg-keep/10',
    unknown: 'text-muted bg-black/5',
  }
</script>

<span class="inline-flex items-center gap-1.5 rounded-sm px-2 py-0.5 text-xs font-medium {tone[verdict]}">
  <Icon name={verdict} size={14} />
  <span class="text-primary">{strings.verdict[verdict]}</span>
  {#if source === 'ai'}
    <span class="text-muted font-normal">· {strings.source.ai}</span>
  {/if}
</span>
