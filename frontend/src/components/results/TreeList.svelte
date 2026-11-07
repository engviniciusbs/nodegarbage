<script lang="ts">
  import { onMount, tick } from 'svelte'
  import TreeRow from './TreeRow.svelte'
  import type { NodeDTO } from '$lib/api'
  import { results, nav, openNode, goUp } from '$lib/stores/results.svelte'
  import { selection, toggle } from '$lib/stores/selection.svelte'
  import { ROW_HEIGHT, visibleRange } from '$lib/virtual'
  import { strings } from '$lib/strings'

  let { items, focusedId = $bindable(null) }: { items: NodeDTO[]; focusedId?: number | null } = $props()

  let el: HTMLDivElement | undefined = $state()
  let scrollTop = $state(0)
  let viewport = $state(0)

  const focusedIndex = $derived(Math.max(0, items.findIndex((n) => n.id === focusedId)))
  const range = $derived(visibleRange(scrollTop, viewport, ROW_HEIGHT, items.length))
  const rendered = $derived(items.slice(range.start, range.end))
  const focusedRendered = $derived(focusedIndex >= range.start && focusedIndex < range.end)

  onMount(() => {
    const measure = () => (viewport = el?.clientHeight ?? 0)
    measure()
    const pending = nav.pendingFocus
    if (pending) {
      nav.pendingFocus = undefined
      const idx = pending.id === null ? -1 : items.findIndex((n) => n.id === pending.id)
      void moveTo(Math.max(0, idx))
    }
    if (typeof ResizeObserver === 'undefined' || !el) return
    const ro = new ResizeObserver(measure)
    ro.observe(el)
    return () => ro.disconnect()
  })

  async function moveTo(i: number) {
    if (!items.length || !el) return
    const idx = Math.max(0, Math.min(items.length - 1, i))
    focusedId = items[idx].id
    const top = idx * ROW_HEIGHT
    if (top < el.scrollTop) el.scrollTop = top
    else if (top + ROW_HEIGHT > el.scrollTop + viewport) el.scrollTop = top + ROW_HEIGHT - viewport
    scrollTop = el.scrollTop
    await tick()
    el.querySelector<HTMLElement>(`[data-index="${idx}"]`)?.focus()
  }

  $effect(() => {
    const id = results.focusRequest
    if (id === null) return
    const idx = items.findIndex((n) => n.id === id)
    results.focusRequest = null
    if (idx >= 0) void moveTo(idx)
  })

  function up() {
    if (results.path.length) goUp(results.path.length - 1)
  }

  function onkeydown(e: KeyboardEvent) {
    // A focused checkbox toggles itself natively on Space.
    if (e.key === ' ' && e.target instanceof HTMLInputElement) return
    const page = Math.max(1, Math.floor(viewport / ROW_HEIGHT) - 1)
    const cur = items[focusedIndex]
    const keys: Record<string, () => void> = {
      ArrowDown: () => moveTo(focusedIndex + 1),
      ArrowUp: () => (e.altKey ? up() : moveTo(focusedIndex - 1)),
      PageDown: () => moveTo(focusedIndex + page),
      PageUp: () => moveTo(focusedIndex - page),
      Home: () => moveTo(0),
      End: () => moveTo(items.length - 1),
      Enter: () => {
        if (cur?.hasChildren) openNode(cur.id)
      },
      ' ': () => {
        if (cur) toggle(cur)
      },
      Backspace: up,
    }
    const act = keys[e.key]
    if (!act) return
    e.preventDefault()
    act()
  }
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div
  bind:this={el}
  role="tree"
  aria-label={strings.results.listLabel}
  tabindex={focusedRendered ? -1 : 0}
  class="relative min-h-0 flex-1 overflow-y-auto focus-visible:-outline-offset-2"
  onscroll={() => (scrollTop = el?.scrollTop ?? 0)}
  {onkeydown}
>
  <div style="height:{items.length * ROW_HEIGHT}px;position:relative">
    {#each rendered as n, k (n.id)}
      <TreeRow
        node={n}
        index={range.start + k}
        total={items.length}
        focused={range.start + k === focusedIndex}
        selected={selection.paths.has(n.path)}
        hovered={results.hoverId === n.id}
        ontoggle={() => toggle(n)}
        onfocus={() => (focusedId = n.id)}
        onopen={() => n.hasChildren && openNode(n.id)}
      />
    {/each}
  </div>
</div>
