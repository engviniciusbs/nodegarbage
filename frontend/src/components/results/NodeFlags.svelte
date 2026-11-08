<script lang="ts">
  import Icon from '../ui/Icon.svelte'
  import type { NodeDTO } from '$lib/api'
  import { strings } from '$lib/strings'

  let { node }: { node: NodeDTO } = $props()
</script>

<!-- No wrapper: the icons and the pill flow inside the caller's flex line. -->
{#if node.link}<span class="shrink-0 text-muted"><Icon name="link" size={14} /></span><span class="sr-only">{strings.results.link}</span>{/if}
{#if node.cloud}<span class="shrink-0 text-muted"><Icon name="cloud" size={14} /></span><span class="sr-only">{strings.results.cloud}</span>{/if}
{#if node.denied}<span class="shrink-0 text-muted"><Icon name="denied" size={14} /></span><span class="sr-only">{strings.results.denied}</span>{/if}
{#if node.inactive}
  <span class="shrink-0 rounded-full border border-inactive-border px-1.5 py-px text-[11px] whitespace-nowrap text-warn-text">{strings.results.inactive}</span>
{/if}
