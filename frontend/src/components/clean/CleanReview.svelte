<script lang="ts">
  import Button from '../ui/Button.svelte'
  import Skeleton from '../ui/Skeleton.svelte'
  import CleanItems from './CleanItems.svelte'
  import { formatBytes } from '$lib/format'
  import { clean, setMode, replan, execute, canConfirm, type CleanMode } from '$lib/stores/clean.svelte'
  import { strings } from '$lib/strings'

  let { onClose }: { onClose: () => void } = $props()

  const modes: { id: CleanMode; label: string }[] = [
    { id: 'recycle', label: strings.clean.recycle },
    { id: 'permanent', label: strings.clean.permanent },
  ]
  const suggest = $derived(
    clean.mode === 'recycle' &&
      (clean.plan?.refused ?? []).some((r) => r.reason === 'exceeds_recycle_bin' || r.reason === 'no_recycle_bin'),
  )
  const total = $derived(clean.plan?.totalBytes ?? 0)
</script>

<fieldset class="flex flex-col gap-1">
  <legend class="mb-1 text-sm font-semibold text-primary">{strings.clean.mode}</legend>
  <div role="radiogroup" aria-label={strings.clean.mode} class="flex flex-wrap gap-4">
    {#each modes as m (m.id)}
      <label class="flex items-center gap-2 text-sm text-primary">
        <input type="radio" name="clean-mode" value={m.id} checked={clean.mode === m.id} onchange={() => setMode(m.id)} />
        {m.label}
      </label>
    {/each}
  </div>
  <p class="text-xs text-muted">{clean.mode === 'recycle' ? strings.clean.modeHint : strings.clean.permanentHint}</p>
  {#if suggest}<p class="text-xs text-warn-text">{strings.clean.suggestPermanent}</p>{/if}
</fieldset>

{#if clean.phase === 'planning'}
  <Skeleton lines={4} label={strings.clean.planning} />
{:else}
  {#if clean.error}
    <div role="alert" class="flex flex-col items-start gap-2 text-sm text-primary">
      <p>{clean.error}</p>
      <Button variant="primary" onclick={replan}>{strings.clean.newPlan}</Button>
    </div>
  {/if}
  {#if clean.plan}
    <CleanItems plan={clean.plan} />
    {#if clean.plan.items.length === 0 && !clean.error}
      <p class="text-sm text-muted">{strings.clean.nothing}</p>
    {/if}
    {#if clean.mode === 'permanent' && clean.plan.items.length > 0}
      <label class="flex items-center gap-2 text-sm font-medium text-primary">
        <input type="checkbox" bind:checked={clean.permanentAck} />
        {strings.clean.understandPermanent}
      </label>
    {/if}
  {/if}
{/if}

<div class="mt-2 flex justify-end gap-2">
  <Button onclick={onClose}>{strings.clean.cancel}</Button>
  <Button variant="primary" disabled={!canConfirm()} onclick={execute}>{strings.clean.confirm(formatBytes(total))}</Button>
</div>
