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

  // Tile tints (tokens.css); anything else (unknown, AI-less) uses --tile-unknown.
  const VERDICT_TOKEN: Record<string, string> = { safe: '--tile-safe', caution: '--tile-caution', keep: '--tile-keep' }

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
    const text = token('--text-primary')
    const muted = token('--text-muted')
    const accent = token('--accent')
    const font = token('--font-sans') || 'sans-serif'

    // The layout leaves a 2px gap between tiles; the canvas background (--tile-gap) shows through it.
    for (const r of rects) {
      const n = byId.get(r.id)
      ctx.fillStyle = token((n && VERDICT_TOKEN[n.verdict]) || '--tile-unknown')
      tile(ctx, r)
      if (n && r.w > 60 && r.h > 22) label(ctx, n, r, text, muted, font)
    }
    for (const id of [focusedId, hoverId]) {
      const r = rects.find((x) => x.id === id)
      if (!r) continue
      ctx.strokeStyle = id === hoverId ? text : accent
      ctx.lineWidth = 2
      ctx.strokeRect(r.x + 1, r.y + 1, r.w - 2, r.h - 2)
    }
  })

  /** Rounded tile (4px, as in the mockup) where the canvas supports it. */
  function tile(ctx: CanvasRenderingContext2D, r: { x: number; y: number; w: number; h: number }) {
    if (typeof ctx.roundRect !== 'function') return ctx.fillRect(r.x, r.y, r.w, r.h)
    ctx.beginPath()
    ctx.roundRect(r.x, r.y, r.w, r.h, Math.min(4, r.w / 2, r.h / 2))
    ctx.fill()
  }

  function fit(ctx: CanvasRenderingContext2D, s: string, max: number): string {
    if (ctx.measureText(s).width <= max) return s
    let end = s.length
    while (end > 1 && ctx.measureText(s.slice(0, end) + '…').width > max) end--
    return s.slice(0, end) + '…'
  }

  function label(
    ctx: CanvasRenderingContext2D,
    n: NodeDTO,
    r: { x: number; y: number; w: number; h: number },
    color: string,
    muted: string,
    font: string,
  ) {
    ctx.textBaseline = 'top'
    const max = r.w - 16
    ctx.fillStyle = color
    ctx.font = `600 12px ${font}`
    ctx.fillText(fit(ctx, n.name, max), r.x + 8, r.y + 6, max)
    if (r.h > 38) {
      ctx.fillStyle = muted
      ctx.font = `12px ${font}`
      ctx.fillText(fit(ctx, formatBytes(n.size), max), r.x + 8, r.y + 22, max)
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
  class="absolute top-0 left-0 block rounded-sm bg-tile-gap {hoverId === null ? '' : 'cursor-pointer'}"
  style="width:{width}px;height:{height}px"
  onpointermove={move}
  onpointerleave={leave}
  onclick={click}
></canvas>
