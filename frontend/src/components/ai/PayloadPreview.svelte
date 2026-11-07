<script lang="ts">
  import Panel from '../ui/Panel.svelte'
  import type { PayloadPreviewDTO } from '$lib/api'
  import { formatCount } from '$lib/format'
  import { strings } from '$lib/strings'

  let { preview }: { preview: PayloadPreviewDTO } = $props()
</script>

<Panel aria-label={strings.ai.previewTitle}>
  <div class="mb-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-sm text-primary">
    <h2 class="font-semibold">{strings.ai.previewTitle}</h2>
    <span class="text-muted">{strings.ai.tokens(formatCount(preview.tokens))}</span>
    <span class="text-muted">{strings.ai.providerModel(preview.provider, preview.model)}</span>
    {#if preview.cached}
      <span class="rounded-sm bg-black/5 px-2 py-0.5 text-xs font-medium text-primary">{strings.ai.cached}</span>
    {/if}
  </div>
  <p class="mb-2 text-xs text-muted">{strings.ai.previewNote}</p>
  <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
  <pre class="max-h-72 overflow-auto rounded-md bg-black/5 p-3 font-mono text-xs whitespace-pre-wrap break-all text-primary" tabindex="0">{preview.json}</pre>
</Panel>
