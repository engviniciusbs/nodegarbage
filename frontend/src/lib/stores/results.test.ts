import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('$lib/api', () => ({ GetChildren: vi.fn(), ListFiltered: vi.fn(), LocatePath: vi.fn() }))

import * as api from '$lib/api'
import { results, refresh, loadVolume } from './results.svelte'

const m = vi.mocked(api, true)

function deferred<T>() {
  let resolve!: (v: T) => void
  const promise = new Promise<T>((r) => (resolve = r))
  return { promise, resolve }
}
const filtered = (count: number) => ({ items: [], count, totalBytes: count * 10, truncated: false })

beforeEach(async () => {
  vi.clearAllMocks()
  m.GetChildren.mockResolvedValue([] as never)
  m.ListFiltered.mockResolvedValue(filtered(0) as never)
  await loadVolume('')
})

describe('results store', () => {
  it('refresh does nothing without a loaded volume', async () => {
    await refresh()
    expect(m.GetChildren).not.toHaveBeenCalled()
    expect(m.ListFiltered).not.toHaveBeenCalled()
  })

  it('a stale shortcut response never overwrites a newer one', async () => {
    results.volume = 'C:\\'
    const first = deferred<unknown>()
    const second = deferred<unknown>()
    m.ListFiltered.mockReturnValueOnce(first.promise as never).mockReturnValueOnce(second.promise as never)
    const r1 = refresh()
    const r2 = refresh()
    second.resolve(filtered(2))
    await r2
    await vi.waitFor(() => expect(results.shortcut?.count).toBe(2))
    first.resolve(filtered(1))
    await r1
    await Promise.resolve()
    expect(results.shortcut?.count).toBe(2)
  })
})
