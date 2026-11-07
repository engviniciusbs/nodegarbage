<script lang="ts">
  import { onMount } from 'svelte'
  import Button from '../components/ui/Button.svelte'
  import EmptyState from '../components/ui/EmptyState.svelte'
  import ErrorState from '../components/ui/ErrorState.svelte'
  import Skeleton from '../components/ui/Skeleton.svelte'
  import PayloadPreview from '../components/ai/PayloadPreview.svelte'
  import ReportView from '../components/ai/ReportView.svelte'
  import { route } from '$lib/route.svelte'
  import {
    ai,
    initAIEvents,
    loadAI,
    selectProfile,
    loadPreview,
    runAnalysis,
    cancelAnalysis,
  } from '$lib/stores/ai.svelte'
  import { strings } from '$lib/strings'

  onMount(() => {
    initAIEvents()
    void loadAI()
  })
</script>

{#if ai.loading}
  <Skeleton lines={6} />
{:else if ai.loadError}
  <ErrorState message={ai.loadError} onretry={loadAI} />
{:else if ai.profiles.length === 0}
  <EmptyState title={strings.ai.noProfilesTitle} description={strings.ai.noProfilesBody} icon="ai">
    {#snippet action()}
      <Button variant="primary" onclick={() => (route.current = 'settings')}>{strings.ai.goSettings}</Button>
    {/snippet}
  </EmptyState>
{:else}
  <div class="flex flex-col gap-3">
    <div class="flex flex-wrap items-end gap-3">
      <label class="flex flex-col gap-1 text-sm text-primary">
        {strings.ai.profile}
        <select
          class="rounded-md border border-glass-border bg-white/70 px-3 py-2 text-sm text-primary"
          value={ai.profileId}
          disabled={ai.running}
          onchange={(e) => selectProfile(e.currentTarget.value)}
        >
          {#each ai.profiles as p (p.id)}
            <option value={p.id}>{p.name} ({p.model})</option>
          {/each}
        </select>
      </label>
      <Button disabled={ai.running || ai.previewLoading} onclick={loadPreview}>{strings.ai.preview}</Button>
      <Button variant="primary" disabled={ai.running} onclick={() => runAnalysis(false)}>
        {ai.running ? strings.ai.running : strings.ai.run}
      </Button>
      {#if ai.report}
        <Button disabled={ai.running} onclick={() => runAnalysis(true)}>{strings.ai.rerun}</Button>
      {/if}
      {#if ai.running}
        <Button variant="ghost" onclick={cancelAnalysis}>{strings.ai.cancel}</Button>
      {/if}
    </div>

    {#if ai.previewLoading}
      <Skeleton lines={4} />
    {:else if ai.previewError}
      <p role="alert" class="text-sm text-warn-text">{ai.previewError}</p>
    {:else if ai.preview}
      <PayloadPreview preview={ai.preview} />
    {/if}

    {#if ai.error}
      <ErrorState title={strings.ai.runError} message={ai.error} onretry={() => runAnalysis(false)} />
    {:else if ai.report}
      <ReportView report={ai.report} />
    {:else if !ai.running}
      <EmptyState title={strings.ai.noReportTitle} description={strings.ai.noReportBody} icon="ai" />
    {/if}
  </div>
{/if}
