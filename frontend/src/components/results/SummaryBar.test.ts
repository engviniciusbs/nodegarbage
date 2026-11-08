import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'

vi.mock('$lib/api', () => ({ ListFiltered: vi.fn(), GetChildren: vi.fn(), LocatePath: vi.fn() }))

import * as api from '$lib/api'
import type { SummaryDTO } from '$lib/api'
import SummaryBar from './SummaryBar.svelte'
import { results, clearFilter, NODE_MODULES_RULE } from '$lib/stores/results.svelte'
import { strings } from '$lib/strings'

const m = vi.mocked(api, true)
const summary = {
  volume: 'C:\\',
  safeBytes: 3 * 1024 ** 3,
  cautionBytes: 512 * 1024 ** 2,
  keepBytes: 2 * 1024 ** 3,
  byCategory: [
    { category: 'dev-cache', bytes: 2 * 1024 ** 3, count: 4 },
    { category: 'weird-new', bytes: 1024 ** 2, count: 1 },
  ],
} as unknown as SummaryDTO

beforeEach(() => {
  vi.clearAllMocks()
  clearFilter()
  results.volume = 'C:\\'
  results.shortcut = null
  m.ListFiltered.mockResolvedValue({ items: [], count: 0, totalBytes: 0, truncated: false } as never)
})

describe('SummaryBar', () => {
  it('shows formatted safe, caution and protected totals', () => {
    render(SummaryBar, { summary })
    // Each figure is a big number followed by its label, read as one line.
    const fig = (label: string) => screen.getByText(label).closest('p')
    expect(fig(strings.results.safeLabel)).toHaveTextContent('3,0 GB recuperáveis com segurança')
    expect(fig(strings.results.cautionLabel)).toHaveTextContent('512,0 MB exigem cuidado')
    expect(fig(strings.results.keepLabel)).toHaveTextContent('2,0 GB protegidos por regras')
    expect(screen.getByRole('region', { name: strings.results.summaryLabel })).toBeInTheDocument()
  })

  it('a category chip toggles the volume-wide filter with aria-pressed', async () => {
    render(SummaryBar, { summary })
    const chip = screen.getByRole('button', { name: /Cache de desenvolvimento/ })
    expect(chip).toHaveAttribute('aria-pressed', 'false')
    await userEvent.click(chip)
    expect(results.filter).toMatchObject({ category: 'dev-cache', ruleId: '' })
    expect(m.ListFiltered).toHaveBeenCalledWith('C:\\', { category: 'dev-cache', ruleId: '' })
    expect(chip).toHaveAttribute('aria-pressed', 'true')
    await userEvent.click(chip)
    expect(results.filter).toBeNull()
  })

  it('shows the node_modules shortcut only when the volume has some', async () => {
    const view = render(SummaryBar, { summary })
    expect(screen.queryByRole('button', { name: /^node_modules/ })).toBeNull()
    results.shortcut = { count: 0, totalBytes: 0 }
    await Promise.resolve()
    expect(screen.queryByRole('button', { name: /^node_modules/ })).toBeNull()
    view.unmount()
    results.shortcut = { count: 3, totalBytes: 20 * 1024 ** 3 }
    render(SummaryBar, { summary })
    const chip = screen.getByRole('button', { name: /^node_modules · 20,0 GB/ })
    await userEvent.click(chip)
    expect(results.filter).toMatchObject({ category: '', ruleId: NODE_MODULES_RULE })
    expect(chip).toHaveAttribute('aria-pressed', 'true')
  })

  it('falls back to the raw category id for unknown categories', () => {
    render(SummaryBar, { summary })
    expect(screen.getByRole('button', { name: /weird-new/ })).toBeInTheDocument()
  })
})
