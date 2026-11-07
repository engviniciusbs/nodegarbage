import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, within } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'

vi.mock('$lib/api', () => ({
  ListVolumes: vi.fn(),
  IsElevated: vi.fn(),
  RestartElevated: vi.fn(),
  StartScan: vi.fn(),
  CancelScan: vi.fn(),
  on: vi.fn(() => () => {}),
}))

import * as api from '$lib/api'
import Home from './Home.svelte'
import { route } from '$lib/route.svelte'
import { resetScan } from '$lib/stores/scan.svelte'
import { strings } from '$lib/strings'

const GB = 1024 ** 3
const vols = [
  { root: 'C:\\', label: 'Windows', fs: 'NTFS', type: 'fixed', total: 100 * GB, free: 40 * GB, hasRecycleBin: true, lastScan: null, estimatedJunk: 5 * GB },
  { root: 'D:\\', label: '', fs: 'exFAT', type: 'fixed', total: 200 * GB, free: 150 * GB, hasRecycleBin: true, lastScan: '2026-01-02T12:00:00Z', estimatedJunk: 0 },
]
const m = vi.mocked(api, true)

beforeEach(() => {
  vi.clearAllMocks()
  resetScan()
  route.current = 'home'
  m.ListVolumes.mockResolvedValue(vols as never)
  m.IsElevated.mockResolvedValue(true)
  m.StartScan.mockResolvedValue(undefined)
})

describe('Home', () => {
  it('shows a skeleton while volumes load', () => {
    m.ListVolumes.mockReturnValue(new Promise(() => {}))
    render(Home)
    expect(screen.getByRole('status', { busy: true })).toBeInTheDocument()
  })

  it('renders a card per volume with label, FS, free space and usage bar', async () => {
    render(Home)
    expect(await screen.findByText('Windows')).toBeInTheDocument()
    expect(screen.getByText('NTFS', { exact: false })).toBeInTheDocument()
    expect(screen.getByText('40,0 GB livres de 100,0 GB')).toBeInTheDocument()
    const bars = screen.getAllByRole('progressbar')
    expect(bars).toHaveLength(2)
    expect(bars[0]).toHaveAttribute('aria-valuenow', '60')
    expect(screen.getByText(strings.home.junk('5,0 GB'))).toBeInTheDocument()
    expect(screen.getByText(strings.home.neverScanned)).toBeInTheDocument()
    expect(screen.getByText(/Última análise/)).toBeInTheDocument()
    expect(screen.getAllByText('D:\\').length).toBeGreaterThan(0)
  })

  it('disables Analisar until a volume is checked, then scans selected roots', async () => {
    render(Home)
    await screen.findByText('Windows')
    const btn = screen.getByRole('button', { name: strings.home.analyze })
    expect(btn).toBeDisabled()
    await userEvent.click(screen.getByRole('checkbox', { name: /Windows/ }))
    expect(btn).toBeEnabled()
    await userEvent.click(btn)
    expect(m.StartScan).toHaveBeenCalledWith(['C:\\'])
    expect(route.current).toBe('scan')
  })

  it('shows the admin banner when not elevated and restarts on click', async () => {
    m.IsElevated.mockResolvedValue(false)
    m.RestartElevated.mockResolvedValue(undefined)
    render(Home)
    const b = await screen.findByRole('button', { name: strings.home.adminRestart })
    await userEvent.click(b)
    expect(m.RestartElevated).toHaveBeenCalled()
  })

  it('hides the admin banner when elevated', async () => {
    render(Home)
    await screen.findByText('Windows')
    expect(screen.queryByRole('button', { name: strings.home.adminRestart })).toBeNull()
  })

  it('shows an inline error when RestartElevated rejects', async () => {
    m.IsElevated.mockResolvedValue(false)
    m.RestartElevated.mockRejectedValue(new Error('uac negado'))
    render(Home)
    await userEvent.click(await screen.findByRole('button', { name: strings.home.adminRestart }))
    expect(await screen.findByText(/uac negado/)).toBeInTheDocument()
  })

  it('shows ErrorState with retry when ListVolumes fails', async () => {
    m.ListVolumes.mockRejectedValueOnce(new Error('boom'))
    render(Home)
    const alert = await screen.findByRole('alert')
    expect(within(alert).getByText(/boom/)).toBeInTheDocument()
    await userEvent.click(within(alert).getByRole('button', { name: strings.common.retry }))
    expect(await screen.findByText('Windows')).toBeInTheDocument()
  })
})
