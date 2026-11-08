<script lang="ts">
  import AIItemGroup from './AIItemGroup.svelte'
  import ReportMarkdown from './ReportMarkdown.svelte'
  import type { AIReportDTO } from '$lib/api'
  import { GROUP_ORDER, groupItems, sumBytes } from '$lib/aireport'
  import { formatBytes } from '$lib/format'
  import { strings } from '$lib/strings'

  let { report }: { report: AIReportDTO } = $props()

  const created = $derived(new Date(report.createdAt).toLocaleString('pt-BR'))
  const groups = $derived(groupItems(report.items ?? []))
  const meta = $derived(
    report.profile || report.model ? strings.ai.reportMeta(report.profile, report.model, created) : strings.ai.createdAt(created),
  )
</script>

<section aria-label={strings.ai.reportTitle} class="flex flex-col gap-4">
  <header class="flex flex-col gap-1 rounded-md border border-stroke bg-card px-4 py-3">
    <p class="m-0 font-display text-lg font-semibold text-primary tabular-nums">
      {strings.ai.totals(formatBytes(sumBytes(groups.safe)), formatBytes(sumBytes(groups.caution)))}
    </p>
    <p class="m-0 text-xs text-muted">{meta} · {strings.ai.untrusted}</p>
  </header>

  {#if report.items?.length}
    {#each GROUP_ORDER as id (id)}
      <AIItemGroup {id} items={groups[id]} />
    {/each}
  {/if}

  <details class="rounded-md border border-stroke bg-card">
    <summary class="cursor-pointer px-4 py-2.5 text-sm font-semibold text-primary">{strings.ai.fullReport}</summary>
    <div class="border-t border-stroke-soft px-4 py-3">
      <ReportMarkdown markdown={report.markdown} />
    </div>
  </details>
</section>
