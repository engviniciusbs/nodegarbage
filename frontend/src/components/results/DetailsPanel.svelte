<script lang="ts">
  import Button from '../ui/Button.svelte'
  import VerdictBadge from '../ui/VerdictBadge.svelte'
  import CopyCommand from './CopyCommand.svelte'
  import FileList from './FileList.svelte'
  import type { NodeDTO } from '$lib/api'
  import { formatBytes, formatCount, formatDate } from '$lib/format'
  import { results, openNode } from '$lib/stores/results.svelte'
  import { strings, type Verdict } from '$lib/strings'

  let { node }: { node?: NodeDTO } = $props()

  const asVerdict = (v: string) => (v in strings.verdict ? v : 'unknown') as Verdict
  const pct = (c: number) => Math.round((c <= 1 ? c * 100 : c) * 10) / 10
  const why = $derived(node?.explain || (node && asVerdict(node.verdict) === 'unknown' ? strings.results.whyUnknown : ''))
  const stats = $derived(
    node
      ? [
          { label: strings.results.size, value: formatBytes(node.size) },
          { label: strings.results.fileCount, value: formatCount(node.files) },
          { label: strings.results.modified, value: node.latestMod ? formatDate(node.latestMod) : '—' },
        ]
      : [],
  )
  const caption = 'mb-1 text-xs font-semibold tracking-[.04em] text-muted uppercase'
</script>

<aside
  aria-label={strings.results.details}
  class="flex min-h-0 flex-col gap-3.5 overflow-y-auto rounded-md border border-stroke bg-card p-4 text-[13px] leading-[1.55]"
>
  {#if !node}
    <p class="text-muted">{strings.results.noFocus}</p>
  {:else}
    <h2 class="m-0 flex flex-wrap items-center gap-2.5 font-display text-lg leading-snug font-semibold break-all text-primary">
      {node.name}
      <VerdictBadge verdict={asVerdict(node.verdict)} source={node.source} />
    </h2>
    <p class="font-mono text-xs break-all text-muted select-text">{node.path}</p>
    <dl class="grid grid-cols-[repeat(3,auto)] gap-1.5">
      {#each stats as s (s.label)}
        <div class="rounded-sm bg-well px-2 py-2">
          <dt class="text-[11px] leading-tight tracking-[.02em] whitespace-nowrap text-muted uppercase">{s.label}</dt>
          <dd class="mt-0.5 text-[13px] font-semibold whitespace-nowrap tabular-nums">{s.value}</dd>
        </div>
      {/each}
    </dl>
    {#if why}
      <section>
        <h3 class={caption}>{strings.results.why}</h3>
        <p class="text-primary">{why}</p>
      </section>
    {/if}
    {#if node.regenerable}
      <section>
        <h3 class={caption}>{strings.results.howToRecreate}</h3>
        <p class="text-primary">{node.regenerable}</p>
      </section>
    {/if}
    {#if node.command}
      <section>
        <h3 class={caption}>{strings.results.command}</h3>
        <CopyCommand command={node.command} />
      </section>
    {/if}
    {#if node.ruleId && node.source !== 'ai'}
      <p class="text-xs text-muted">
        {strings.results.rule} <code class="font-mono">{node.ruleId}</code>{#if node.action === 'delete_contents'}
          · {strings.results.ruleContents}{/if}
      </p>
    {/if}
    {#if node.ai}
      <section class="flex flex-col gap-1.5 rounded-sm bg-well p-3">
        <h3 class={caption}>{strings.results.aiTitle}</h3>
        <span><VerdictBadge verdict={asVerdict(node.ai.verdict)} /></span>
        <p class="text-primary">{node.ai.reason}</p>
        <p class="text-xs text-muted">{strings.results.aiConfidence(pct(node.ai.confidence))}</p>
        <p class="text-xs text-muted">{strings.results.aiNote}</p>
      </section>
    {/if}
    {#if node.hasChildren}
      <div><Button onclick={() => openNode(node.id)}>{strings.results.openFolder}</Button></div>
    {/if}
    {#if node.files > 0}
      <FileList volume={results.volume} id={node.id} />
    {/if}
  {/if}
</aside>
