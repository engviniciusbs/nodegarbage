<script lang="ts">
  import Panel from '../ui/Panel.svelte'
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
</script>

<Panel aria-label={strings.results.details} class="space-y-3 overflow-y-auto text-sm">
  {#if !node}
    <p class="text-muted">{strings.results.noFocus}</p>
  {:else}
    <h2 class="text-base font-semibold break-all text-primary">{node.name}</h2>
    <p class="font-mono text-xs break-all text-muted select-text">{node.path}</p>
    <dl class="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1">
      <dt class="text-muted">{strings.results.size}</dt>
      <dd class="tabular-nums">{formatBytes(node.size)}</dd>
      <dt class="text-muted">{strings.results.fileCount}</dt>
      <dd class="tabular-nums">{formatCount(node.files)}</dd>
      <dt class="text-muted">{strings.results.lastModified}</dt>
      <dd>{node.latestMod ? formatDate(node.latestMod) : '—'}</dd>
    </dl>
    <VerdictBadge verdict={asVerdict(node.verdict)} source={node.source} />
    {#if node.explain}<p class="text-primary">{node.explain}</p>{/if}
    {#if node.regenerable}
      <div>
        <h3 class="font-medium text-primary">{strings.results.howToRecreate}</h3>
        <p class="text-muted">{node.regenerable}</p>
      </div>
    {/if}
    {#if node.command}
      <div class="space-y-1">
        <h3 class="font-medium text-primary">{strings.results.command}</h3>
        <CopyCommand command={node.command} />
      </div>
    {/if}
    {#if node.ai}
      <div class="space-y-1 rounded-md bg-black/5 p-3">
        <h3 class="font-medium text-primary">{strings.results.aiTitle}</h3>
        <VerdictBadge verdict={asVerdict(node.ai.verdict)} />
        <p class="text-primary">{node.ai.reason}</p>
        <p class="text-xs text-muted">{strings.results.aiConfidence(pct(node.ai.confidence))}</p>
        <p class="text-xs text-muted">{strings.results.aiNote}</p>
      </div>
    {/if}
    {#if node.hasChildren}
      <Button variant="secondary" onclick={() => openNode(node.id)}>{strings.results.openFolder}</Button>
    {/if}
    {#if node.files > 0}
      <FileList volume={results.volume} id={node.id} />
    {/if}
  {/if}
</Panel>
