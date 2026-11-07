<script lang="ts">
  import { untrack } from 'svelte'
  import Button from '../ui/Button.svelte'
  import { SaveAIProfile, TestAIProfile } from '$lib/api'
  import type { AIProfileDTO } from '$lib/api'
  import { strings } from '$lib/strings'

  let { profile, onSaved }: { profile?: AIProfileDTO; onSaved: (p: AIProfileDTO) => void } = $props()

  // The form edits a local copy: the initial profile is read once on purpose.
  const initial = untrack(() => profile)
  const OPENAI_URL = 'https://api.openai.com/v1'
  let id = $state(initial?.id ?? '')
  let hasKey = $state(initial?.hasKey ?? false)
  let name = $state(initial?.name ?? '')
  let kind = $state(initial?.kind ?? 'openai-compat')
  let baseUrl = $state(initial?.baseUrl || (initial?.kind === 'anthropic' ? '' : OPENAI_URL))
  let model = $state(initial?.model ?? '')
  let apiKey = $state('')
  let errors = $state({ name: '', model: '', baseUrl: '' })
  let busy = $state<'' | 'save' | 'test'>('')
  let failure = $state('')
  let success = $state('')

  const isAnthropic = $derived(kind === 'anthropic')

  function onKindChange() {
    if (isAnthropic) baseUrl = ''
    else if (!baseUrl) baseUrl = OPENAI_URL
  }

  function validate(): boolean {
    errors.name = name.trim() ? '' : strings.profile.nameRequired
    errors.model = model.trim() ? '' : strings.profile.modelRequired
    errors.baseUrl = ''
    if (!isAnthropic) {
      const u = baseUrl.trim()
      if (!u) errors.baseUrl = strings.profile.urlRequired
      else if (!/^https?:\/\//i.test(u)) errors.baseUrl = strings.profile.urlInvalid
    }
    return !errors.name && !errors.model && !errors.baseUrl
  }

  async function persist(): Promise<AIProfileDTO | null> {
    if (!validate()) return null
    const dto = { id, name: name.trim(), kind, baseUrl: isAnthropic ? '' : baseUrl.trim(), model: model.trim(), hasKey } as AIProfileDTO
    try {
      const saved = await SaveAIProfile(dto, apiKey)
      id = saved.id
      hasKey = saved.hasKey
      apiKey = '' // the key never stays in the DOM after saving
      onSaved(saved)
      return saved
    } catch (e) {
      failure = `${strings.profile.saveError} ${e instanceof Error ? e.message : String(e)}`
      return null
    }
  }

  async function run(mode: 'save' | 'test') {
    busy = mode
    failure = ''
    success = ''
    const saved = await persist()
    if (saved && mode === 'test') {
      try {
        await TestAIProfile(saved.id)
        success = strings.profile.testOk
      } catch (e) {
        failure = e instanceof Error ? e.message : String(e)
      }
    }
    busy = ''
  }

  const field = 'w-full rounded-md border border-glass-border bg-white/70 px-3 py-2 text-sm text-primary'
</script>

<form class="flex flex-col gap-3" onsubmit={(e) => { e.preventDefault(); void run('save') }} novalidate>
  <label class="flex flex-col gap-1 text-sm text-primary">
    {strings.profile.name}
    <input class={field} bind:value={name} required aria-invalid={errors.name ? 'true' : undefined} aria-describedby={errors.name ? 'pf-name-err' : undefined} />
    {#if errors.name}<span id="pf-name-err" class="text-xs text-warn-text">{errors.name}</span>{/if}
  </label>
  <label class="flex flex-col gap-1 text-sm text-primary">
    {strings.profile.kind}
    <select class={field} bind:value={kind} onchange={onKindChange}>
      <option value="openai-compat">{strings.profile.kindOpenAI}</option>
      <option value="anthropic">{strings.profile.kindAnthropic}</option>
    </select>
  </label>
  {#if !isAnthropic}
    <label class="flex flex-col gap-1 text-sm text-primary">
      {strings.profile.baseUrl}
      <input class={field} type="url" bind:value={baseUrl} aria-invalid={errors.baseUrl ? 'true' : undefined} aria-describedby={errors.baseUrl ? 'pf-url-err' : undefined} />
      {#if errors.baseUrl}<span id="pf-url-err" class="text-xs text-warn-text">{errors.baseUrl}</span>{/if}
    </label>
  {/if}
  <label class="flex flex-col gap-1 text-sm text-primary">
    {strings.profile.model}
    <input class={field} bind:value={model} required aria-invalid={errors.model ? 'true' : undefined} aria-describedby={errors.model ? 'pf-model-err' : undefined} />
    {#if errors.model}<span id="pf-model-err" class="text-xs text-warn-text">{errors.model}</span>{/if}
  </label>
  <label class="flex flex-col gap-1 text-sm text-primary">
    {strings.profile.apiKey}
    <input class={field} type="password" autocomplete="off" bind:value={apiKey} placeholder={hasKey ? strings.profile.keyKept : ''} />
  </label>
  <div class="flex flex-wrap items-center gap-2">
    <Button type="submit" variant="primary" disabled={busy !== ''}>{busy === 'save' ? strings.profile.saving : strings.profile.save}</Button>
    <Button disabled={busy !== ''} onclick={() => run('test')}>{busy === 'test' ? strings.profile.testing : strings.profile.test}</Button>
    <span class="text-xs text-muted">{strings.profile.testHint}</span>
  </div>
  <div aria-live="polite" class="text-sm">
    {#if success}<p class="text-primary">{success}</p>{/if}
    {#if failure}<p role="alert" class="text-warn-text">{failure}</p>{/if}
  </div>
</form>
