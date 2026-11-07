import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'

vi.mock('$lib/api', () => ({
  GetChildren: vi.fn(),
  GetSummary: vi.fn(),
  ListFiles: vi.fn(),
  ListVolumes: vi.fn(),
  StartScan: vi.fn(),
  CancelScan: vi.fn(),
  on: vi.fn(() => () => {}),
}))

import * as api from '$lib/api'
import Results from './Results.svelte'
import { route } from '$lib/route.svelte'
import { results, loadVolume } from '$lib/stores/results.svelte'
import { resetScan } from '$lib/stores/scan.svelte'
import { selection, clear } from '$lib/stores/selection.svelte'
import { node } from '../test/fixtures'
import { formatBytes } from '$lib/format'
import { strings } from '$lib/strings'

const m = vi.mocked(api, true)
const summary = { volume: 'C:\\', safeBytes: 1, cautionBytes: 1, keepBytes: 1, byCategory: [] }
const rootKids = [node({ id: 1, name: 'alfa' }), node({ id: 2, name: 'beta' })]
const childKids = [node({ id: 10, name: 'filho-a' }), node({ id: 11, name: 'filho-b' })]

let clientHeight: PropertyDescriptor | undefined
beforeEach(async () => {
  vi.clearAllMocks()
  clear()
  clientHeight = Object.getOwnPropertyDescriptor(HTMLElement.prototype, 'clientHeight')
  Object.defineProperty(HTMLElement.prototype, 'clientHeight', { configurable: true, get: () => 400 })
  m.GetSummary.mockResolvedValue(summary as never)
  m.ListVolumes.mockResolvedValue([] as never)
  m.StartScan.mockResolvedValue(undefined)
  resetScan()
  route.current = 'results'
  m.GetChildren.mockImplementation(async (_v, id) => (id === -1 ? rootKids : childKids))
  await loadVolume('')
  route.volume = 'C:\\'
})
afterEach(() => {
  if (clientHeight) Object.defineProperty(HTMLElement.prototype, 'clientHeight', clientHeight)
})

describe('Results screen', () => {
  it('keeps DOM focus in the list after Enter and Backspace', async () => {
    render(Results)
    const first = (await screen.findAllByRole('treeitem'))[0]
    // beta (index 1) is the folder we will drill into.
    await userEvent.click(screen.getAllByRole('treeitem')[1])
    screen.getAllByRole('treeitem')[1].focus()
    await userEvent.keyboard('{Enter}')
    await waitFor(() => expect(screen.getAllByRole('treeitem')[0]).toHaveTextContent('filho-a'))
    await waitFor(() => {
      const active = document.activeElement as HTMLElement
      expect(active.getAttribute('role')).toBe('treeitem')
      expect(active).toHaveTextContent('filho-a')
    })
    expect(first).not.toBeInTheDocument()

    await userEvent.keyboard('{Backspace}')
    await waitFor(() => expect(screen.getAllByRole('treeitem')).toHaveLength(2))
    await waitFor(() => {
      const active = document.activeElement as HTMLElement
      expect(active.getAttribute('role')).toBe('treeitem')
      expect(active).toHaveTextContent('beta')
    })
  })

  it('keeps selection and path when remounted for the same volume', async () => {
    const view = render(Results)
    await screen.findAllByRole('treeitem')
    await userEvent.click(screen.getByRole('checkbox', { name: 'Selecionar alfa' }))
    expect(selection.paths.size).toBe(1)
    view.unmount()
    render(Results)
    await screen.findAllByRole('treeitem')
    expect(selection.paths.size).toBe(1)
    expect(results.volume).toBe('C:\\')
    expect(screen.getByRole('checkbox', { name: 'Selecionar alfa' })).toBeChecked()
  })

  it('clears selection when the volume actually changes', async () => {
    render(Results)
    await screen.findAllByRole('treeitem')
    await userEvent.click(screen.getByRole('checkbox', { name: 'Selecionar alfa' }))
    route.volume = 'D:\\'
    await waitFor(() => expect(selection.paths.size).toBe(0))
  })

  it('Lista / Mapa tabs switch the panel between the list and the treemap', async () => {
    const { container } = render(Results)
    await screen.findAllByRole('treeitem')
    const list = screen.getByRole('tab', { name: strings.results.viewList })
    const map = screen.getByRole('tab', { name: strings.results.viewMap })
    expect(list).toHaveAttribute('aria-selected', 'true')
    expect(container.querySelector('canvas')).toBeNull()

    await userEvent.click(map)
    expect(map).toHaveAttribute('aria-selected', 'true')
    expect(list).toHaveAttribute('aria-selected', 'false')
    expect(screen.getByRole('tabpanel')).toHaveAttribute('aria-labelledby', map.id)
    expect(container.querySelector('canvas')).not.toBeNull()
    expect(screen.queryAllByRole('treeitem')).toHaveLength(0)
    expect(screen.getByRole('list', { name: strings.results.legendLabel })).toBeInTheDocument()

    // Arrow keys move between tabs too.
    map.focus()
    await userEvent.keyboard('{ArrowLeft}')
    expect(list).toHaveAttribute('aria-selected', 'true')
    expect(list).toHaveFocus()
    expect(container.querySelector('canvas')).toBeNull()
    expect(screen.getAllByRole('treeitem')).toHaveLength(2)
  })

  it('footer shows the selected count and bytes, and Limpar seleção clears it', async () => {
    render(Results)
    await screen.findAllByRole('treeitem')
    const footer = screen.getByText(strings.results.noneSelected)
    expect(footer).toBeInTheDocument()
    const clearBtn = screen.getByRole('button', { name: strings.results.clearSelection })
    expect(clearBtn).toBeDisabled()

    await userEvent.click(screen.getByRole('checkbox', { name: 'Selecionar alfa' }))
    await userEvent.click(screen.getByRole('checkbox', { name: 'Selecionar beta' }))
    const size = formatBytes(2 * 1024 * 1024)
    const bar = screen.getByText(strings.results.selectedCount(2)).closest('p')!
    expect(bar).toHaveTextContent(`${strings.results.selectedCount(2)} · ${size}`)
    expect(screen.getByRole('button', { name: strings.clean.button(2, size) })).toBeEnabled()

    await userEvent.click(clearBtn)
    expect(selection.paths.size).toBe(0)
    expect(screen.getByText(strings.results.noneSelected)).toBeInTheDocument()
    expect(screen.getByRole('checkbox', { name: 'Selecionar alfa' })).not.toBeChecked()
    expect(screen.getByRole('button', { name: strings.clean.button(0, formatBytes(0)) })).toBeDisabled()
  })

  it('header shows the volume and Analisar de novo starts a scan of it', async () => {
    m.ListVolumes.mockResolvedValue([{ root: 'C:\\', lastScan: '2026-11-06T11:58:00' }] as never)
    render(Results)
    expect(await screen.findByRole('heading', { level: 1, name: strings.nav.results })).toBeInTheDocument()
    expect(screen.getByText(strings.results.metaVolume('C:\\'))).toBeInTheDocument()
    expect(await screen.findByText(/analisado em 06\/11\/2026 às 11:58/)).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: strings.results.rescan }))
    expect(m.StartScan).toHaveBeenCalledWith(['C:\\'])
    expect(route.current).toBe('scan')
  })

  it('opens the most recently scanned volume when none is chosen', async () => {
    m.ListVolumes.mockResolvedValue([
      { root: 'C:\\', lastScan: '2026-11-01T10:00:00Z' },
      { root: 'D:\\', lastScan: null },
      { root: 'F:\\', lastScan: '2026-11-05T10:00:00Z' },
    ] as never)
    route.volume = ''
    render(Results)
    await waitFor(() => expect(route.volume).toBe('F:\\'))
    expect(await screen.findAllByRole('treeitem')).toHaveLength(2)
    expect(m.GetChildren).toHaveBeenCalledWith('F:\\', -1)
    expect(screen.queryByText(strings.results.emptyVolume)).toBeNull()
  })

  it('shows the empty state only when no volume has a scan', async () => {
    m.ListVolumes.mockResolvedValue([{ root: 'C:\\', lastScan: null }] as never)
    route.volume = ''
    render(Results)
    expect(await screen.findByText(strings.results.emptyVolume)).toBeInTheDocument()
    expect(route.volume).toBe('')
  })
})
