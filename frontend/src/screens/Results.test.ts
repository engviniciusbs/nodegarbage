import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'

vi.mock('$lib/api', () => ({
  GetChildren: vi.fn(),
  ListFiltered: vi.fn(),
  LocatePath: vi.fn(),
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
const noMatches = { items: [], count: 0, totalBytes: 0, truncated: false }

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
  m.ListFiltered.mockResolvedValue(noMatches as never)
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

describe('Results filtered view', () => {
  const nm1 = node({ id: 21, name: 'node_modules', path: 'C:\\a\\node_modules', location: 'C:\\a', size: 3 * 1024 ** 3 })
  const nm2 = node({ id: 22, name: 'node_modules', path: 'C:\\b\\web\\node_modules', location: 'C:\\b\\web', size: 1024 ** 3 })
  const matches = { items: [nm1, nm2], count: 2, totalBytes: 4 * 1024 ** 3, truncated: false }
  const withCats = { ...summary, byCategory: [{ category: 'build-artifact', bytes: 4 * 1024 ** 3, count: 2 }] }

  beforeEach(() => {
    m.GetSummary.mockResolvedValue(withCats as never)
    m.ListFiltered.mockResolvedValue(matches as never)
  })

  async function enterFilter() {
    render(Results)
    await screen.findAllByRole('treeitem')
    await userEvent.click(await screen.findByRole('button', { name: /^Artefatos de build/ }))
    await waitFor(() => expect(screen.getAllByRole('treeitem')[0]).toHaveTextContent('C:\\a'))
  }

  it('a category chip lists the matches of the whole volume with a Local column', async () => {
    await enterFilter()
    expect(m.ListFiltered).toHaveBeenCalledWith('C:\\', { category: 'build-artifact', ruleId: '' })
    const rows = screen.getAllByRole('treeitem')
    expect(rows).toHaveLength(2)
    expect(within(rows[1]).getByTitle('C:\\b\\web')).toHaveTextContent('C:\\b\\web')
    expect(screen.getByText(strings.results.filterSummary('Artefatos de build', 2, formatBytes(4 * 1024 ** 3)))).toBeInTheDocument()
    expect(screen.getByText(strings.results.colLocation)).toBeInTheDocument()
    expect(screen.queryByText(strings.results.truncated)).toBeNull()
  })

  it('shows the node_modules shortcut when the volume has matches', async () => {
    render(Results)
    expect(await screen.findByRole('button', { name: /^node_modules · 4,0 GB/ })).toBeInTheDocument()
    expect(m.ListFiltered).toHaveBeenCalledWith('C:\\', { category: '', ruleId: 'node.node_modules' })
  })

  it('hides the shortcut when there are none', async () => {
    m.ListFiltered.mockResolvedValue(noMatches as never)
    render(Results)
    await screen.findAllByRole('treeitem')
    await waitFor(() => expect(m.ListFiltered).toHaveBeenCalled())
    expect(screen.queryByRole('button', { name: /^node_modules/ })).toBeNull()
  })

  it('says when the list was capped', async () => {
    m.ListFiltered.mockResolvedValue({ ...matches, count: 9000, truncated: true } as never)
    await enterFilter()
    expect(screen.getByText(strings.results.truncated)).toBeInTheDocument()
  })

  it('keeps the selection and ✕ returns to the folder navigation', async () => {
    await enterFilter()
    await userEvent.click(screen.getByRole('checkbox', { name: strings.results.selectAt('node_modules', 'C:\\a') }))
    expect(selection.paths.has('C:\\a\\node_modules')).toBe(true)
    await userEvent.click(screen.getByRole('button', { name: strings.results.clearFilter }))
    await waitFor(() => expect(screen.getAllByRole('treeitem')[0]).toHaveTextContent('alfa'))
    expect(results.filter).toBeNull()
    expect(selection.paths.size).toBe(1)
    expect(screen.getByText(strings.results.selectedCount(1))).toBeInTheDocument()
  })

  it('Enter on a filtered row opens its folder with the item focused', async () => {
    const a = node({ id: 30, name: 'a', path: 'C:\\a' })
    m.LocatePath.mockResolvedValue({ volume: 'C:\\', chain: [a, nm1] } as never)
    m.GetChildren.mockImplementation(async (_v, id) => (id === 30 ? [nm1] : rootKids))
    await enterFilter()
    screen.getAllByRole('treeitem')[0].focus()
    await userEvent.keyboard('{Enter}')
    await waitFor(() => expect(results.filter).toBeNull())
    expect(m.LocatePath).toHaveBeenCalledWith('C:\\a\\node_modules')
    expect(results.path.map((n) => n.id)).toEqual([30])
    await waitFor(() => expect(document.activeElement).toHaveTextContent('node_modules'))
  })

  it('the map tab shows the filtered items', async () => {
    const { container } = await (async () => {
      const v = render(Results)
      await screen.findAllByRole('treeitem')
      await userEvent.click(await screen.findByRole('button', { name: /^Artefatos de build/ }))
      return v
    })()
    await userEvent.click(screen.getByRole('tab', { name: strings.results.viewMap }))
    expect(container.querySelector('canvas')).not.toBeNull()
    expect(screen.getByText(strings.results.filterSummary('Artefatos de build', 2, formatBytes(4 * 1024 ** 3)))).toBeInTheDocument()
  })
})
