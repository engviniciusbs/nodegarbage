import { hierarchy, treemap, treemapSquarify } from 'd3-hierarchy'

export type Rect = { id: number; x: number; y: number; w: number; h: number }

type Datum = { id: number; size: number; children?: Datum[] }

/** Squarified treemap of `nodes`; zero-size nodes are omitted. */
export function computeTreemap(nodes: { id: number; size: number }[], width: number, height: number): Rect[] {
  const kept = nodes.filter((n) => n.size > 0)
  if (kept.length === 0 || width <= 0 || height <= 0) return []
  const root = hierarchy<Datum>({ id: -1, size: 0, children: kept.map((n) => ({ id: n.id, size: n.size })) })
    .sum((d) => (d.children ? 0 : d.size))
    .sort((a, b) => (b.value ?? 0) - (a.value ?? 0))
  treemap<Datum>().tile(treemapSquarify).size([width, height]).padding(2)(root)
  return (root.children ?? []).map((n) => {
    const r = n as unknown as { x0: number; y0: number; x1: number; y1: number }
    return { id: n.data.id, x: r.x0, y: r.y0, w: r.x1 - r.x0, h: r.y1 - r.y0 }
  })
}

export function hitTest(rects: Rect[], x: number, y: number): number | null {
  for (const r of rects) if (x >= r.x && x < r.x + r.w && y >= r.y && y < r.y + r.h) return r.id
  return null
}
