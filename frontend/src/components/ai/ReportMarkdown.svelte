<script lang="ts">
  import { renderMarkdown } from '$lib/markdown'

  let { markdown }: { markdown: string } = $props()

  const html = $derived(renderMarkdown(markdown))
</script>

<!-- renderMarkdown output is DOMPurify-sanitized; this is the only {@html} in the app. -->
<div class="prose-report text-sm text-primary">{@html html}</div>

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
