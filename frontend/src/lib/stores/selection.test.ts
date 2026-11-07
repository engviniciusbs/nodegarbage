import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('$lib/api', () => ({ GetChildren: vi.fn() }))

import * as api from '$lib/api'
import { selection, toggle, clear, selectedBytes } from './selection.svelte'
import { results, loadVolume, openNode } from './results.svelte'
import { node } from '../../test/fixtures'

const m = vi.mocked(api, true)

beforeEach(() => {
  vi.clearAllMocks()
  clear()
})

describe('selection', () => {
  it('toggles a selectable node on and off', () => {
    const n = node({ id: 1 })
    toggle(n)
    expect(selection.paths.has(n.path)).toBe(true)
    toggle(n)
    expect(selection.paths.size).toBe(0)
  })

  it('ignores keep, manual and link nodes', () => {
    toggle(node({ id: 1, verdict: 'keep' }))
    toggle(node({ id: 2, action: 'manual' }))
    toggle(node({ id: 3, link: true }))
    expect(selection.paths.size).toBe(0)
  })

  it('sums selected bytes', () => {
    toggle(node({ id: 1, size: 100 }))
    toggle(node({ id: 2, size: 250 }))
    expect(selectedBytes()).toBe(350)
  })

  it('never pre-selects: loading and opening nodes selects nothing', async () => {
    const root = [node({ id: 1 }), node({ id: 2 })]
    m.GetChildren.mockResolvedValueOnce(root).mockResolvedValueOnce([node({ id: 3 })])
    await loadVolume('C:\\')
    await openNode(1)
    expect(results.path.map((p) => p.id)).toEqual([1])
    expect(selection.paths.size).toBe(0)
  })
})
