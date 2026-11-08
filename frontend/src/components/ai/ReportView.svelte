<script lang="ts">
  import Panel from '../ui/Panel.svelte'
  import VerdictBadge from '../ui/VerdictBadge.svelte'
  import type { AIReportDTO } from '$lib/api'
  import { renderMarkdown } from '$lib/markdown'
  import { ai, revealItem } from '$lib/stores/ai.svelte'
  import { strings, type Verdict } from '$lib/strings'

  let { report }: { report: AIReportDTO } = $props()

  const html = $derived(renderMarkdown(report.markdown))
  const verdict = (v: string): Verdict => (v in strings.verdict ? (v as Verdict) : 'unknown')
  const created = $derived(new Date(report.createdAt).toLocaleString('pt-BR'))
</script>

<Panel aria-label={strings.ai.reportTitle}>
  <p class="mb-2 text-xs text-muted">{strings.ai.createdAt(created)} · {strings.ai.untrusted}</p>
  <!-- renderMarkdown output is DOMPurify-sanitized; this is the only {@html} in the app. -->
  <div class="prose-report text-sm text-primary">{@html html}</div>
</Panel>

{#if report.items?.length}
  <section class="mt-3" aria-label={strings.ai.itemsTitle}>
    <h3 class="mb-2 text-sm font-semibold text-primary">{strings.ai.itemsTitle}</h3>
    <ul class="flex flex-col gap-2">
      {#each report.items as item (item.path)}
        <li class="flex flex-col gap-1 rounded-md border border-stroke bg-card px-3 py-2">
          <div class="flex flex-wrap items-center gap-2">
            <VerdictBadge verdict={verdict(item.verdict)} source="ai" />
            <button
              type="button"
              class="cursor-pointer rounded-sm px-1 text-left font-mono text-xs break-all text-accent hover:underline"
              aria-label={strings.ai.reveal(item.path)}
              onclick={() => revealItem(item.path)}>{item.path}</button
            >
            <span class="ml-auto text-xs text-muted">{strings.ai.confidence(Math.round(item.confidence * 100))}</span>
          </div>
          <p class="text-sm text-primary">{item.reason}</p>
          {#if ai.revealErrors[item.path]}
            <p role="alert" class="text-xs text-warn-text">{ai.revealErrors[item.path]}</p>
          {/if}
        </li>
      {/each}
    </ul>
  </section>
{/if}

<style>
  .prose-report :global(h1),
  .prose-report :global(h2),
  .prose-report :global(h3),
  .prose-report :global(h4) {
    font-weight: 600;
    margin: 0.9em 0 0.4em;
    color: var(--text-primary);
  }
  .prose-report :global(h1) { font-size: 1.25rem; }
  .prose-report :global(h2) { font-size: 1.1rem; }
  .prose-report :global(p) { margin: 0.5em 0; }
  .prose-report :global(ul) { list-style: disc; padding-left: 1.4em; margin: 0.5em 0; }
  .prose-report :global(ol) { list-style: decimal; padding-left: 1.4em; margin: 0.5em 0; }
  .prose-report :global(code) {
    font-family: var(--font-mono);
    font-size: 0.85em;
    background: color-mix(in srgb, var(--text-primary) 8%, transparent);
    padding: 0.1em 0.3em;
    border-radius: var(--radius-sm);
  }
  .prose-report :global(pre) { overflow: auto; padding: 0.6em; border-radius: var(--radius-md); background: color-mix(in srgb, var(--text-primary) 6%, transparent); }
  .prose-report :global(pre code) { background: none; padding: 0; }
  .prose-report :global(blockquote) { border-left: 3px solid var(--stroke); padding-left: 0.8em; color: var(--text-muted); }
  .prose-report :global(table) { border-collapse: collapse; }
  .prose-report :global(th),
  .prose-report :global(td) { border: 1px solid var(--stroke); padding: 0.25em 0.6em; }
</style>
