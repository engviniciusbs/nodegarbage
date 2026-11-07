<script lang="ts" module>
  export type ResultsView = 'list' | 'map'
</script>

<script lang="ts">
  import { strings } from '$lib/strings'

  let { view = $bindable('list'), controls }: { view?: ResultsView; controls: string } = $props()

  const tabs: { id: ResultsView; label: string }[] = [
    { id: 'list', label: strings.results.viewList },
    { id: 'map', label: strings.results.viewMap },
  ]
  let el: HTMLDivElement | undefined = $state()

  function onkeydown(e: KeyboardEvent) {
    if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight' && e.key !== 'Home' && e.key !== 'End') return
    e.preventDefault()
    const i = tabs.findIndex((t) => t.id === view)
    const next = e.key === 'Home' ? 0 : e.key === 'End' ? tabs.length - 1 : (i + (e.key === 'ArrowRight' ? 1 : -1) + tabs.length) % tabs.length
    view = tabs[next].id
    el?.querySelector<HTMLElement>(`#results-tab-${view}`)?.focus()
  }
</script>

<div
  bind:this={el}
  role="tablist"
  tabindex="-1"
  aria-label={strings.results.viewLabel}
  class="inline-flex shrink-0 gap-0.5 rounded-md border border-stroke bg-card p-[3px]"
  {onkeydown}
>
  {#each tabs as t (t.id)}
    {@const active = view === t.id}
    <button
      type="button"
      role="tab"
      id="results-tab-{t.id}"
      aria-selected={active}
      aria-controls={controls}
      tabindex={active ? 0 : -1}
      onclick={() => (view = t.id)}
      class="cursor-pointer rounded-[7px] px-3 py-1.5 text-[13px] {active
        ? 'bg-field font-semibold text-primary shadow-sm'
        : 'text-muted hover:text-primary'}">{t.label}</button
    >
  {/each}
</div>
