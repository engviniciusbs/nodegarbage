<script lang="ts">
  import Button from '../ui/Button.svelte'
  import type { FileDTO } from '$lib/api'
  import { ListFiles } from '$lib/api'
  import { formatBytes } from '$lib/format'
  import { strings, type Verdict } from '$lib/strings'

  const LIMIT = 200

  let { volume, id }: { volume: string; id: number } = $props()

  let open = $state(false)
  let loading = $state(false)
  let error = $state('')
  let files = $state<FileDTO[]>([])

  // A different folder always starts collapsed.
  $effect(() => {
    void id
    open = false
    files = []
    error = ''
  })

  async function toggle() {
    open = !open
    if (!open || files.length) return
    loading = true
    error = ''
    try {
      files = (await ListFiles(volume, id)) ?? []
    } catch (e) {
      error = `${strings.results.filesError} ${e instanceof Error ? e.message : String(e)}`
    } finally {
      loading = false
    }
  }

  const label = (v: string) => strings.verdict[(v in strings.verdict ? v : 'unknown') as Verdict]
</script>

<div class="space-y-2">
  <Button variant="secondary" aria-expanded={open} onclick={toggle}>
    {open ? strings.results.hideFiles : strings.results.showFiles}
  </Button>
  {#if open}
    {#if loading}
      <p class="text-sm text-muted" role="status">{strings.results.filesLoading}</p>
    {:else if error}
      <p class="text-sm text-primary" role="alert">{error}</p>
    {:else if files.length === 0}
      <p class="text-sm text-muted">{strings.results.filesEmpty}</p>
    {:else}
      <ul aria-label={strings.results.filesLabel} class="max-h-64 space-y-1 overflow-y-auto text-xs">
        {#each files.slice(0, LIMIT) as f (f.path)}
          <li class="flex items-center gap-2">
            <span class="min-w-0 flex-1 truncate" title={f.path}>{f.name}</span>
            <span class="tabular-nums">{formatBytes(f.size)}</span>
            <span class="w-20 text-right text-muted">{label(f.verdict)}</span>
          </li>
        {/each}
      </ul>
      {#if files.length > LIMIT}
        <p class="text-xs text-muted">{strings.results.filesLimit(LIMIT, files.length)}</p>
      {/if}
    {/if}
  {/if}
</div>
