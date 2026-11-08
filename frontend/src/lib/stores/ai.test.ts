import { describe, it, expect, vi, beforeEach } from 'vitest'

const handlers: Record<string, (d: unknown) => void> = {}
vi.mock('$lib/api', () => ({
  GetAIReport: vi.fn(),
  ListAIProfiles: vi.fn(),
  PreviewAIPayload: vi.fn(),
  RunAI: vi.fn(),
  CancelAI: vi.fn(),
  LocatePath: vi.fn(),
  GetChildren: vi.fn(),
  ListFiltered: vi.fn(),
  on: (ev: string, cb: (d: unknown) => void) => {
    handlers[ev] = cb
    return () => {}
  },
}))

import * as api from '$lib/api'
import { ai, reloadReport, initAIEvents } from './ai.svelte'

const m = vi.mocked(api, true)
const rep = (markdown: string) => ({ markdown, items: [], createdAt: '2026-11-07T10:00:00Z', profile: '', model: '' })

function deferred<T>() {
  let resolve!: (v: T) => void
  const promise = new Promise<T>((r) => (resolve = r))
  return { promise, resolve }
}

beforeEach(() => {
  vi.clearAllMocks()
  ai.report = null
  initAIEvents()
})

describe('ai store reloadReport', () => {
  it('a stale reload never overwrites a newer one', async () => {
    const first = deferred<unknown>()
    const second = deferred<unknown>()
    m.GetAIReport.mockReturnValueOnce(first.promise as never).mockReturnValueOnce(second.promise as never)
    const r1 = reloadReport()
    const r2 = reloadReport()
    second.resolve(rep('novo'))
    await r2
    first.resolve(rep('velho'))
    await r1
    expect(ai.report?.markdown).toBe('novo')
  })

  it('a fresh ai:done report wins over a reload still in flight', async () => {
    const pending = deferred<unknown>()
    m.GetAIReport.mockReturnValueOnce(pending.promise as never)
    const r = reloadReport()
    handlers['ai:done']({ report: rep('analise'), error: '' })
    pending.resolve(rep('velho'))
    await r
    expect(ai.report?.markdown).toBe('analise')
  })
})
