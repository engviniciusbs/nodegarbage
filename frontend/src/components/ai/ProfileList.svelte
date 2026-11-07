<script lang="ts">
  import Button from '../ui/Button.svelte'
  import Panel from '../ui/Panel.svelte'
  import EmptyState from '../ui/EmptyState.svelte'
  import ProfileForm from './ProfileForm.svelte'
  import { ListAIProfiles, DeleteAIProfile } from '$lib/api'
  import type { AIProfileDTO } from '$lib/api'
  import { strings } from '$lib/strings'

  let { onChange }: { onChange?: (profiles: AIProfileDTO[]) => void } = $props()

  let profiles = $state<AIProfileDTO[]>([])
  let editing = $state<AIProfileDTO | 'new' | null>(null)
  let error = $state('')

  async function reload() {
    try {
      profiles = (await ListAIProfiles()) ?? []
      onChange?.(profiles)
    } catch (e) {
      error = e instanceof Error ? e.message : String(e)
    }
  }
  void reload()

  async function remove(p: AIProfileDTO) {
    if (!window.confirm(strings.profile.removeConfirm(p.name))) return
    error = ''
    try {
      await DeleteAIProfile(p.id)
      if (editing !== 'new' && editing?.id === p.id) editing = null
      await reload()
    } catch (e) {
      error = `${strings.profile.removeError} ${e instanceof Error ? e.message : String(e)}`
    }
  }
</script>

<div class="flex flex-col gap-3">
  {#if profiles.length === 0}
    <EmptyState title={strings.profile.empty} />
  {:else}
    <ul aria-label={strings.profile.listLabel} class="flex flex-col gap-2">
      {#each profiles as p (p.id)}
        <li class="glass flex items-center justify-between gap-3 rounded-md px-3 py-2">
          <span class="min-w-0 text-sm text-primary">
            <span class="font-medium">{p.name}</span>
            <span class="text-muted"> · {p.model}</span>
          </span>
          <span class="flex gap-1">
            <Button variant="ghost" aria-label={strings.profile.edit(p.name)} onclick={() => (editing = p)}>{strings.profile.editLabel}</Button>
            <Button variant="ghost" aria-label={strings.profile.remove(p.name)} onclick={() => remove(p)}>{strings.profile.removeLabel}</Button>
          </span>
        </li>
      {/each}
    </ul>
  {/if}
  {#if error}<p role="alert" class="text-sm text-warn-text">{error}</p>{/if}
  {#if editing}
    <Panel>
      {#key editing === 'new' ? 'new' : editing.id}
        <ProfileForm profile={editing === 'new' ? undefined : editing} onSaved={() => void reload()} />
      {/key}
    </Panel>
  {:else}
    <div><Button variant="primary" onclick={() => (editing = 'new')}>{strings.profile.add}</Button></div>
  {/if}
</div>
