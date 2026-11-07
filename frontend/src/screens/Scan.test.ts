import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, within } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import { tick } from 'svelte'

const handlers: Record<string, (d: unknown) => void> = {}
vi.mock('$lib/api', () => ({
  StartScan: vi.fn(),
  CancelScan: vi.fn(),
  on: vi.fn((ev: string, cb: (d: unknown) => void) => {
    handlers[ev] = cb
    return () => {}
  }),
}))

import * as api from '$lib/api'
import Scan from './Scan.svelte'
import { route } from '$lib/route.svelte'
import { scanState, startScan, resetScan } from '$lib/stores/scan.svelte'
import { strings } from '$lib/strings'

const m = vi.mocked(api, true)
const prog = (volume: string) => ({
  volume, files: 1234, dirs: 56, bytes: 2 * 1024 ** 3, denied: 0, current: 'C:\\Users\\x\\file.txt', bytesPerSec: 1048576,
})
const doneDto = (volume: string, error = '') => ({ volume, durationMs: 10, error, deniedCount: 0, deniedSample: [] })

beforeEach(async () => {
  vi.clearAllMocks()
  resetScan()
  route.current = 'scan'
  route.volume = ''
  m.StartScan.mockResolvedValue(undefined)
  await startScan(['C:\\', 'D:\\'])
})

describe('scan store + Scan screen', () => {
  it('shows formatted counters in a polite live region', async () => {
    render(Scan)
    handlers['scan:progress'](prog('C:\\'))
    await tick()
    expect(screen.getByText('1.234')).toBeInTheDocument()
    expect(screen.getByText('2,0 GB')).toBeInTheDocument()
    expect(screen.getByText('1,0 MB/s')).toBeInTheDocument()
    const live = document.querySelector('[aria-live="polite"]')
    expect(live).not.toBeNull()
    expect(live!.textContent).toContain('1.234')
    expect(screen.getAllByText(strings.scan.waiting).length).toBe(1)
  })

  it('Cancelar calls CancelScan', async () => {
    render(Scan)
    await userEvent.click(screen.getByRole('button', { name: strings.scan.cancel }))
    expect(m.CancelScan).toHaveBeenCalled()
  })

  it('goes to results after the last done without errors', async () => {
    render(Scan)
    handlers['scan:done'](doneDto('C:\\'))
    await tick()
    expect(route.current).toBe('scan')
    handlers['scan:done'](doneDto('D:\\'))
    await tick()
    expect(scanState.running).toBe(false)
    expect(route.current).toBe('results')
    expect(route.volume).toBe('C:\\')
  })

  it('shows ErrorState with message and back button on error', async () => {
    render(Scan)
    handlers['scan:done'](doneDto('C:\\', 'disco removido'))
    handlers['scan:done'](doneDto('D:\\'))
    const alert = await screen.findByRole('alert')
    expect(within(alert).getByText(/disco removido/)).toBeInTheDocument()
    expect(route.current).toBe('scan')
    await userEvent.click(screen.getByRole('button', { name: strings.scan.back }))
    expect(route.current).toBe('home')
  })

  it('shows canceled message and back button', async () => {
    render(Scan)
    handlers['scan:done'](doneDto('C:\\', 'canceled'))
    await tick()
    expect(await screen.findByText(strings.scan.canceledBody)).toBeInTheDocument()
    expect(scanState.running).toBe(false)
    await userEvent.click(screen.getByRole('button', { name: strings.scan.back }))
    expect(route.current).toBe('home')
  })
})
