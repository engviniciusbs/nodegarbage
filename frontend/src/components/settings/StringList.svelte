<script lang="ts">
  import Button from '../ui/Button.svelte'
  import { strings } from '$lib/strings'

  let {
    items = $bindable(),
    inputLabel,
    addLabel,
    listLabel,
    emptyLabel,
  }: { items: string[]; inputLabel: string; addLabel: string; listLabel: string; emptyLabel: string } = $props()

  let draft = $state('')

  function add() {
    const v = draft.trim()
    if (v && !items.includes(v)) items = [...items, v]
    draft = ''
  }
</script>

<div class="flex flex-col gap-2">
  <div class="flex items-end gap-2">
    <label class="flex min-w-0 flex-1 flex-col gap-1 text-sm text-primary">
      {inputLabel}
      <input
        class="w-full rounded-md border border-glass-border bg-white/70 px-3 py-2 text-sm text-primary"
        bind:value={draft}
        onkeydown={(e) => {
          if (e.key === 'Enter') {
            e.preventDefault()
            add()
          }
        }}
      />
    </label>
    <Button onclick={add} disabled={!draft.trim()}>{addLabel}</Button>
  </div>
  {#if items.length === 0}
    <p class="text-xs text-muted">{emptyLabel}</p>
  {:else}
    <ul aria-label={listLabel} class="flex flex-wrap gap-2">
      {#each items as v (v)}
        <li class="glass flex items-center gap-1 rounded-md py-1 pl-3 pr-1 text-sm text-primary">
          <span class="break-all font-mono text-xs">{v}</span>
          <Button variant="ghost" class="!px-2 !py-0.5" aria-label={strings.settings.remove(v)} onclick={() => (items = items.filter((x) => x !== v))}>×</Button>
        </li>
      {/each}
    </ul>
  {/if}
</div>
