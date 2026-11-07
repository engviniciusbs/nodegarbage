<script lang="ts">
  import VerdictBadge from '../ui/VerdictBadge.svelte'
  import { formatBytes } from '$lib/format'
  import { riskAck, needsRisk } from '$lib/stores/clean.svelte'
  import { refusalText, warningText, baseName } from '$lib/reasons'
  import { strings, type Verdict } from '$lib/strings'
  import type { PlanDTO } from '$lib/api'

  /** Rows rendered for plain safe items; items that need the risk checkbox are always all rendered. */
  const CAP = 100
  let { plan }: { plan: PlanDTO } = $props()

  const risky = $derived(plan.items.filter(needsRisk))
  const safe = $derived(plan.items.filter((i) => !needsRisk(i)))
  const shownSafe = $derived(safe.slice(0, CAP))
  const hidden = $derived(safe.length - shownSafe.length)
</script>

{#if plan.items.length > 0}
  <section class="flex flex-col gap-2">
    <h3 class="text-sm font-semibold text-primary">{strings.clean.itemsTitle(plan.items.length)}</h3>
    <ul aria-label={strings.clean.itemsLabel} class="flex max-h-64 flex-col gap-1 overflow-auto">
      {#each [...risky, ...shownSafe] as it (it.path)}
        <li class="glass flex flex-col gap-1 rounded-md px-3 py-2 text-sm">
          <div class="flex items-center justify-between gap-3">
            <span class="min-w-0 break-all font-mono text-xs text-primary">{it.path}</span>
            <span class="flex shrink-0 items-center gap-2">
              <VerdictBadge verdict={it.verdict as Verdict} source={it.source} />
              <span class="tabular-nums text-primary">{formatBytes(it.size)}</span>
            </span>
          </div>
          {#each it.warnings ?? [] as w (w)}
            <p class="text-xs text-warn-text">{warningText(w)}</p>
          {/each}
          {#if needsRisk(it)}
            <label class="flex items-center gap-2 text-primary">
              <input
                type="checkbox"
                checked={riskAck.has(it.path)}
                aria-label={strings.clean.understandRiskFor(baseName(it.path))}
                onchange={(e) => (e.currentTarget.checked ? riskAck.add(it.path) : riskAck.delete(it.path))}
              />
              {strings.clean.understandRisk}
            </label>
          {/if}
        </li>
      {/each}
      {#if hidden > 0}<li class="px-3 py-1 text-xs text-muted">{strings.clean.more(hidden)}</li>{/if}
    </ul>
  </section>
{/if}

{#if plan.refused.length > 0}
  <section class="flex flex-col gap-2">
    <h3 class="text-sm font-semibold text-primary">{strings.clean.refusedTitle(plan.refused.length)}</h3>
    <ul aria-label={strings.clean.refusedLabel} class="flex max-h-40 flex-col gap-1 overflow-auto">
      {#each plan.refused as r (r.path)}
        <li class="rounded-md bg-black/5 px-3 py-2 text-sm">
          <span class="break-all font-mono text-xs text-primary">{r.path}</span>
          <p class="text-xs text-muted">{refusalText(r.reason)}</p>
        </li>
      {/each}
    </ul>
  </section>
{/if}
