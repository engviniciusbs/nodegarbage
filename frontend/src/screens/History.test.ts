import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, within } from '@testing-library/svelte'

vi.mock('$lib/api', () => ({ GetHistory: vi.fn() }))

import * as api from '$lib/api'
import History from './History.svelte'
import { formatBytes } from '$lib/format'
import { strings } from '$lib/strings'

const m = vi.mocked(api, true)
const row = (over: Record<string, unknown> = {}) => ({
  path: 'C:\\proj\\node_modules',
  size: 5 * 1024 * 1024,
  mode: 'recycle',
  source: 'rule',
  ruleId: 'r1',
  status: 'ok',
  reason: '',
  freed: 5 * 1024 * 1024,
  at: '2026-11-06T10:00:00Z',
  ...over,
})

beforeEach(() => vi.clearAllMocks())

describe('History', () => {
  it('shows total freed and a table with path, size, mode, date and status', async () => {
    m.GetHistory.mockResolvedValue({
      totalFreed: 7 * 1024 * 1024,
      items: [row(), row({ path: 'C:\\x', status: 'skipped', reason: 'locked', mode: 'permanent' })],
    } as never)
    render(History)
    expect(await screen.findByText(formatBytes(7 * 1024 * 1024))).toBeInTheDocument()
    expect(m.GetHistory).toHaveBeenCalledWith(200)
    const table = screen.getByRole('table', { name: strings.history.tableLabel })
    const path = within(table).getByText('C:\\proj\\node_modules')
    expect(path.className).toContain('font-mono')
    expect(within(table).getByText(strings.history.recycle)).toBeInTheDocument()
    expect(within(table).getByText(strings.history.permanent)).toBeInTheDocument()
    expect(within(table).getByText(strings.history.ok)).toBeInTheDocument()
    expect(within(table).getByText(strings.history.skipped)).toBeInTheDocument()
    expect(within(table).getByText(strings.refusal.locked)).toBeInTheDocument()
    expect(within(table).getAllByText('06/11/2026').length).toBeGreaterThan(0)
  })

  it('shows the empty state when there is no history', async () => {
    m.GetHistory.mockResolvedValue({ totalFreed: 0, items: [] } as never)
    render(History)
    expect(await screen.findByText(strings.history.emptyTitle)).toBeInTheDocument()
  })

  it('shows an error with retry', async () => {
    m.GetHistory.mockRejectedValue('boom')
    render(History)
    expect(await screen.findByRole('alert')).toHaveTextContent('boom')
  })
})
