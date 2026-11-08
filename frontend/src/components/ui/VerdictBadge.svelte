<script lang="ts">
  import Icon from './Icon.svelte'
  import { strings, type Verdict } from '$lib/strings'

  let { verdict, source, compact = false }: { verdict: Verdict; source?: string; compact?: boolean } = $props()

  // Icon + label (never colour alone). Verdict colours pass 4.5:1 on their own tint (tokens.css).
  const tone: Record<Verdict, string> = {
    safe: 'text-safe bg-tint-safe border-transparent',
    caution: 'text-warn-text bg-tint-caution border-transparent',
    keep: 'text-keep bg-tint-keep border-transparent',
    unknown: 'text-muted bg-transparent border-dashed border-unknown-border font-medium',
  }
</script>

<span
  class="inline-flex max-w-full items-center gap-[5px] justify-self-start rounded-full border py-0.5 pr-[9px] pl-[7px] text-xs font-semibold whitespace-nowrap {tone[verdict]}"
  title={source === 'ai' && compact ? strings.source.ai : undefined}
>
  <Icon name={verdict} size={13} />
  <span class="truncate">{strings.verdict[verdict]}</span>
  {#if source === 'ai'}
    {#if compact}
      <span class="font-normal" aria-hidden="true">· {strings.source.aiShort}</span>
      <span class="sr-only">{strings.source.ai}</span>
    {:else}
      <span class="font-normal">· {strings.source.ai}</span>
    {/if}
  {/if}
</span>
