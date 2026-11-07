<script lang="ts">
  import { onMount } from 'svelte'
  import Button from '../components/ui/Button.svelte'
  import Panel from '../components/ui/Panel.svelte'
  import ErrorState from '../components/ui/ErrorState.svelte'
  import Skeleton from '../components/ui/Skeleton.svelte'
  import ProfileList from '../components/ai/ProfileList.svelte'
  import StringList from '../components/settings/StringList.svelte'
  import { GetSettings, SaveSettings } from '$lib/api'
  import type { SettingsDTO } from '$lib/api'
  import { strings } from '$lib/strings'

  const MB = 1024 * 1024
  const field = 'w-40 rounded-md border border-glass-border bg-white/70 px-3 py-2 text-sm text-primary'

  let loaded = $state<SettingsDTO | null>(null)
  let loading = $state(true)
  let loadError = $state('')
  let terms = $state<string[]>([])
  let protectedPaths = $state<string[]>([])
  let minMb = $state('0')
  let workers = $state('0')
  let saving = $state(false)
  let message = $state<{ ok: boolean; text: string } | null>(null)

  const msg = (e: unknown) => (typeof e === 'string' ? e : e instanceof Error ? e.message : String(e))

  async function load() {
    loading = true
    loadError = ''
    try {
      const s = await GetSettings()
      loaded = s
      terms = [...(s.redactTerms ?? [])]
      protectedPaths = [...(s.extraProtected ?? [])]
      minMb = String(Math.round((s.aiMinBytes ?? 0) / MB))
      workers = String(s.workers ?? 0)
    } catch (e) {
      loadError = `${strings.settings.loadError} ${msg(e)}`
    } finally {
      loading = false
    }
  }
  onMount(() => void load())

  async function save(e: Event) {
    e.preventDefault()
    const mb = Number(minMb)
    const w = Number(workers)
    if (!Number.isInteger(mb) || mb < 0 || !Number.isInteger(w) || w < 0) {
      message = { ok: false, text: strings.settings.invalidNumber }
      return
    }
    saving = true
    message = null
    try {
      await SaveSettings({ ...loaded!, redactTerms: terms, extraProtected: protectedPaths, aiMinBytes: mb * MB, workers: w } as SettingsDTO)
      message = { ok: true, text: strings.settings.saved }
    } catch (err) {
      message = { ok: false, text: `${strings.settings.saveError} ${msg(err)}` }
    } finally {
      saving = false
    }
  }
</script>

<div class="flex flex-col gap-4">
  <Panel>
    <h2 class="mb-3 text-base font-semibold text-primary">{strings.settings.aiTitle}</h2>
    <ProfileList />
  </Panel>

  {#if loading}
    <Skeleton lines={5} />
  {:else if loadError}
    <ErrorState message={loadError} onretry={load} />
  {:else if loaded}
    <form class="flex flex-col gap-4" onsubmit={save} novalidate>
      <Panel>
        <h2 class="text-base font-semibold text-primary">{strings.settings.privacyTitle}</h2>
        <p class="mb-3 text-sm text-muted">{strings.settings.privacyHelp}</p>
        <StringList bind:items={terms} inputLabel={strings.settings.termLabel} addLabel={strings.settings.termAdd} listLabel={strings.settings.termsList} emptyLabel={strings.settings.termsEmpty} />
      </Panel>
      <Panel>
        <h2 class="text-base font-semibold text-primary">{strings.settings.protectTitle}</h2>
        <p class="mb-3 text-sm text-muted">{strings.settings.protectHelp}</p>
        <StringList bind:items={protectedPaths} inputLabel={strings.settings.pathLabel} addLabel={strings.settings.pathAdd} listLabel={strings.settings.pathsList} emptyLabel={strings.settings.pathsEmpty} />
      </Panel>
      <Panel class="flex flex-col gap-3">
        <h2 class="text-base font-semibold text-primary">{strings.settings.advancedTitle}</h2>
        <label class="flex flex-col gap-1 text-sm text-primary">
          {strings.settings.aiMinMb}
          <input class={field} type="number" min="0" step="1" bind:value={minMb} />
        </label>
        <label class="flex flex-col gap-1 text-sm text-primary">
          {strings.settings.workers}
          <input class={field} type="number" min="0" step="1" bind:value={workers} />
        </label>
        <label class="flex flex-col gap-1 text-sm text-primary">
          {strings.settings.userRulesDir}
          <input class="w-full rounded-md border border-glass-border bg-black/5 px-3 py-2 font-mono text-xs text-primary" readonly value={loaded.userRulesDir} />
          <span class="text-xs text-muted">{strings.settings.restartNote}</span>
        </label>
      </Panel>
      <div class="flex items-center gap-3">
        <Button type="submit" variant="primary" disabled={saving}>{saving ? strings.settings.saving : strings.settings.save}</Button>
        <div aria-live="polite" class="text-sm">
          {#if message}
            <p role={message.ok ? 'status' : 'alert'} class={message.ok ? 'text-primary' : 'text-warn-text'}>{message.text}</p>
          {/if}
        </div>
      </div>
    </form>

    <Panel>
      <h2 class="mb-2 text-base font-semibold text-primary">{strings.settings.problemsTitle}</h2>
      {#if (loaded.ruleProblems ?? []).length === 0}
        <p class="text-sm text-muted">{strings.settings.problemsEmpty}</p>
      {:else}
        <ul class="flex flex-col gap-1">
          {#each loaded.ruleProblems as p, i (p.file + p.ruleId + i)}
            <li class="text-sm text-warn-text">
              <span class="font-mono text-xs">{strings.settings.problemLine(p.file, p.error)}</span>
              {#if p.ruleId}<span class="text-muted"> ({p.ruleId})</span>{/if}
            </li>
          {/each}
        </ul>
      {/if}
    </Panel>
  {/if}
</div>
