import { describe, it, expect, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import type { SummaryDTO } from '$lib/api'
import SummaryBar from './SummaryBar.svelte'
import { results } from '$lib/stores/results.svelte'

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
  results.categoryFilter = null
})

describe('SummaryBar', () => {
  it('shows formatted safe, caution and protected totals', () => {
    render(SummaryBar, { summary })
    expect(screen.getByText('3,0 GB recuperáveis com segurança')).toBeInTheDocument()
    expect(screen.getByText('512,0 MB exigem cuidado')).toBeInTheDocument()
    expect(screen.getByText('2,0 GB protegidos por regras')).toBeInTheDocument()
  })

  it('category chip toggles categoryFilter with aria-pressed', async () => {
    render(SummaryBar, { summary })
    const chip = screen.getByRole('button', { name: /Cache de desenvolvimento/ })
    expect(chip).toHaveAttribute('aria-pressed', 'false')
    await userEvent.click(chip)
    expect(results.categoryFilter).toBe('dev-cache')
    expect(chip).toHaveAttribute('aria-pressed', 'true')
    await userEvent.click(chip)
    expect(results.categoryFilter).toBeNull()
  })

  it('falls back to the raw category id for unknown categories', () => {
    render(SummaryBar, { summary })
    expect(screen.getByRole('button', { name: /weird-new/ })).toBeInTheDocument()
  })
})
