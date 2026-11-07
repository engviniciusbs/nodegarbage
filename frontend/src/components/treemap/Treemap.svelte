<script lang="ts">
  import { onMount } from 'svelte'
  import { computeTreemap, hitTest } from './layout'
  import type { NodeDTO } from '$lib/api'
  import { formatBytes } from '$lib/format'

  let {
    nodes,
    focusedId = null,
    onOpen,
    onSelect,
    onHover,
  }: {
    nodes: NodeDTO[]
    focusedId?: number | null
    onOpen: (id: number) => void
    onSelect: (id: number) => void
    onHover: (id: number | null) => void
  } = $props()

  const VERDICT_TOKEN: Record<string, string> = { safe: '--safe', caution: '--warn', keep: '--keep' }

  let canvas: HTMLCanvasElement | undefined = $state()
  let width = $state(0)
  let height = $state(0)
  let dpr = $state(1)
  let hoverId = $state<number | null>(null)
  let theme = $state(0) // bumped when the OS colour scheme flips, to redraw with fresh tokens

  $effect(() => {
    void nodes
    hoverId = null
  })

  const rects = $derived(computeTreemap(nodes, width, height))
  const byId = $derived(new Map(nodes.map((n) => [n.id, n])))

  onMount(() => {
    const parent = canvas?.parentElement
    const measure = () => {
      if (!parent) return
      width = parent.clientWidth
      height = parent.clientHeight
      dpr = window.devicePixelRatio || 1
    }
    measure()
    const ro = typeof ResizeObserver === 'undefined' ? undefined : new ResizeObserver(measure)
    if (parent) ro?.observe(parent)
    const mq = window.matchMedia?.('(prefers-color-scheme: dark)')
    const onTheme = () => theme++
    mq?.addEventListener('change', onTheme)
    return () => {
      ro?.disconnect()
      mq?.removeEventListener('change', onTheme)
    }
  })

  $effect(() => {
    void theme
    const el = canvas
    const ctx = el?.getContext('2d') // null under jsdom
    if (!el || !ctx) return
    el.width = Math.max(1, Math.round(width * dpr))
    el.height = Math.max(1, Math.round(height * dpr))
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
    ctx.clearRect(0, 0, width, height)
    const css = getComputedStyle(document.documentElement)
    const token = (name: string) => css.getPropertyValue(name).trim()
    const surface = token('--surface')
    const text = token('--text-primary')
    const accent = token('--accent')
    const font = token('--font-sans') || 'sans-serif'

    for (const r of rects) {
      const n = byId.get(r.id)
      const t = n && VERDICT_TOKEN[n.verdict]
      ctx.globalAlpha = t ? 0.75 : 0.35
      ctx.fillStyle = t ? token(t) : accent
      ctx.fillRect(r.x, r.y, r.w, r.h)
      ctx.globalAlpha = 1
      ctx.strokeStyle = surface
      ctx.lineWidth = 1
      ctx.strokeRect(r.x + 0.5, r.y + 0.5, r.w - 1, r.h - 1)
      if (n && r.w > 60 && r.h > 18) label(ctx, n, r, text, font)
    }
    for (const id of [focusedId, hoverId]) {
      const r = rects.find((x) => x.id === id)
      if (!r) continue
      ctx.strokeStyle = id === hoverId ? text : accent
      ctx.lineWidth = 2
      ctx.strokeRect(r.x + 1, r.y + 1, r.w - 2, r.h - 2)
    }
  })

  function fit(ctx: CanvasRenderingContext2D, s: string, max: number): string {
    if (ctx.measureText(s).width <= max) return s
    let end = s.length
    while (end > 1 && ctx.measureText(s.slice(0, end) + '…').width > max) end--
    return s.slice(0, end) + '…'
  }

  function label(ctx: CanvasRenderingContext2D, n: NodeDTO, r: { x: number; y: number; w: number; h: number }, color: string, font: string) {
    ctx.fillStyle = color
    ctx.textBaseline = 'top'
    const max = r.w - 12
    ctx.font = `600 12px ${font}`
    ctx.fillText(fit(ctx, n.name, max), r.x + 6, r.y + 5, max)
    if (r.h > 34) {
      ctx.font = `12px ${font}`
      ctx.fillText(fit(ctx, formatBytes(n.size), max), r.x + 6, r.y + 21, max)
    }
  }

  function pick(e: PointerEvent | MouseEvent): number | null {
    const box = canvas!.getBoundingClientRect()
    return hitTest(rects, e.clientX - box.left, e.clientY - box.top)
  }

  function move(e: PointerEvent) {
    const id = pick(e)
    if (id === hoverId) return
    hoverId = id
    onHover(id)
  }

  function leave() {
    hoverId = null
    onHover(null)
  }

  function click(e: MouseEvent) {
    const id = pick(e)
    if (id === null) return
    if (byId.get(id)?.hasChildren) onOpen(id)
    else onSelect(id)
  }
</script>

<canvas
  bind:this={canvas}
  aria-hidden="true"
  class="block size-full {hoverId === null ? '' : 'cursor-pointer'}"
  style="width:{width}px;height:{height}px"
  onpointermove={move}
  onpointerleave={leave}
  onclick={click}
></canvas>
