import { describe, expect, it } from 'vitest'
import { computeTreemap, hitTest } from './layout'

const nodes = [
  { id: 1, size: 50 },
  { id: 2, size: 30 },
  { id: 3, size: 20 },
]

describe('computeTreemap', () => {
  it('fills the area (minus padding) and gives the largest node the largest rect', () => {
    const rects = computeTreemap(nodes, 100, 100)
    expect(rects).toHaveLength(3)
    const area = rects.reduce((s, r) => s + r.w * r.h, 0)
    expect(area).toBeGreaterThan(100 * 100 * 0.85)
    expect(area).toBeLessThanOrEqual(100 * 100 * 1.05)
    const biggest = rects.reduce((a, b) => (b.w * b.h > a.w * a.h ? b : a))
    expect(biggest.id).toBe(1)
  })

  it('omits zero-size nodes and handles all-zero or empty input', () => {
    const rects = computeTreemap([...nodes, { id: 4, size: 0 }], 100, 100)
    expect(rects.map((r) => r.id).sort()).toEqual([1, 2, 3])
    expect(computeTreemap([{ id: 1, size: 0 }], 100, 100)).toEqual([])
    expect(computeTreemap([], 100, 100)).toEqual([])
    expect(computeTreemap(nodes, 0, 100)).toEqual([])
  })
})

describe('hitTest', () => {
  it('returns the id under the point, or null outside', () => {
    const rects = computeTreemap(nodes, 100, 100)
    const big = rects.find((r) => r.id === 1)!
    expect(hitTest(rects, big.x + big.w / 2, big.y + big.h / 2)).toBe(1)
    expect(hitTest(rects, 500, 500)).toBeNull()
  })
})
