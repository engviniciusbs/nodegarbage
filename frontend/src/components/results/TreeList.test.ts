import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, within } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import { tick } from 'svelte'

vi.mock('$lib/api', () => ({ GetChildren: vi.fn(), ListFiles: vi.fn() }))

import * as api from '$lib/api'
import TreeList from './TreeList.svelte'
import DetailsPanel from './DetailsPanel.svelte'
import { results, loadVolume } from '$lib/stores/results.svelte'
import { selection, clear } from '$lib/stores/selection.svelte'
import { node } from '../../test/fixtures'
import { formatBytes } from '$lib/format'
import { strings } from '$lib/strings'

const m = vi.mocked(api, true)
const many = (n: number) => Array.from({ length: n }, (_, i) => node({ id: i + 1, size: (n - i) * 1000 }))

let clientHeight: PropertyDescriptor | undefined
beforeEach(async () => {
  vi.clearAllMocks()
  clear()
  clientHeight = Object.getOwnPropertyDescriptor(HTMLElement.prototype, 'clientHeight')
  Object.defineProperty(HTMLElement.prototype, 'clientHeight', { configurable: true, get: () => 400 })
  m.GetChildren.mockResolvedValue([])
  await loadVolume('C:\\')
})
afterEach(() => {
  if (clientHeight) Object.defineProperty(HTMLElement.prototype, 'clientHeight', clientHeight)
})

describe('TreeList', () => {
  it('renders only a window of 5000 rows', () => {
    render(TreeList, { items: many(5000) })
    expect(screen.getByRole('tree')).toBeInTheDocument()
    const rows = screen.getAllByRole('treeitem')
    expect(rows.length).toBeLessThanOrEqual(40)
    expect(rows.length).toBeGreaterThan(5)
    expect(rows[0]).toHaveAttribute('aria-level', '1')
    expect(rows[0]).toHaveAttribute('aria-setsize', '5000')
  })

  it('row shows badge, size and the inactive chip', () => {
    render(TreeList, { items: [node({ id: 1, size: 1536 * 1024, inactive: true, verdict: 'caution' })] })
    const row = screen.getByRole('treeitem')
    expect(within(row).getByText(strings.verdict.caution)).toBeInTheDocument()
    expect(within(row).getByText(formatBytes(1536 * 1024))).toBeInTheDocument()
    expect(within(row).getByText(strings.results.inactive)).toBeInTheDocument()
  })

  it('arrow keys move focus, Home/End jump', async () => {
    render(TreeList, { items: many(50) })
    const rows = () => screen.getAllByRole('treeitem')
    rows()[0].focus()
    await userEvent.keyboard('{ArrowDown}{ArrowDown}')
    expect(rows()[2]).toHaveAttribute('aria-selected', 'true')
    expect(rows()[2]).toHaveFocus()
    await userEvent.keyboard('{ArrowUp}')
    expect(rows()[1]).toHaveAttribute('aria-selected', 'true')
    await userEvent.keyboard('{End}')
    await tick()
    expect(screen.getByText('item-50')).toBeInTheDocument()
    await userEvent.keyboard('{Home}')
    await tick()
    expect(screen.getAllByRole('treeitem')[0]).toHaveAttribute('aria-selected', 'true')
  })

  it('Space toggles selection; keep rows have a disabled checkbox', async () => {
    render(TreeList, { items: [node({ id: 1 }), node({ id: 2, verdict: 'keep' })] })
    screen.getAllByRole('treeitem')[0].focus()
    await userEvent.keyboard(' ')
    expect(selection.paths.size).toBe(1)
    expect(screen.getByRole('checkbox', { name: 'Selecionar item-1' })).toBeChecked()
    await userEvent.keyboard(' ')
    expect(selection.paths.size).toBe(0)
    const keepBox = screen.getByRole('checkbox', { name: 'Selecionar item-2' })
    expect(keepBox).toBeDisabled()
    expect(keepBox).toHaveAttribute('title', strings.results.blocked.keep)
  })

  it('Enter opens a folder; Backspace goes up', async () => {
    const root = [node({ id: 7, name: 'pasta' })]
    m.GetChildren.mockResolvedValueOnce(root)
    await loadVolume('C:\\')
    m.GetChildren.mockResolvedValueOnce([node({ id: 8 })])
    render(TreeList, { items: results.children })
    screen.getAllByRole('treeitem')[0].focus()
    await userEvent.keyboard('{Enter}')
    await tick()
    expect(m.GetChildren).toHaveBeenLastCalledWith('C:\\', 7)
    expect(results.path.map((p) => p.id)).toEqual([7])
    m.GetChildren.mockResolvedValueOnce(root)
    await userEvent.keyboard('{Backspace}')
    await tick()
    expect(m.GetChildren).toHaveBeenLastCalledWith('C:\\', -1)
    expect(results.path).toEqual([])
  })
})

describe('DetailsPanel', () => {
  it('shows the command in <code> and copies it', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText } })
    render(DetailsPanel, { node: node({ command: 'npm cache clean --force' }) })
    const code = screen.getByText('npm cache clean --force')
    expect(code.tagName).toBe('CODE')
    await userEvent.click(screen.getByRole('button', { name: strings.results.copy }))
    expect(writeText).toHaveBeenCalledWith('npm cache clean --force')
    expect(await screen.findByText(strings.results.copied)).toBeInTheDocument()
  })

  it('survives a clipboard rejection', async () => {
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText: vi.fn().mockRejectedValue(new Error('no')) },
    })
    render(DetailsPanel, { node: node({ command: 'x' }) })
    await userEvent.click(screen.getByRole('button', { name: strings.results.copy }))
    expect(await screen.findByText(strings.results.copyError)).toBeInTheDocument()
  })

  it('shows the AI opinion as plain text with confidence', () => {
    render(DetailsPanel, {
      node: node({ ai: { verdict: 'safe', reason: '<b>pode apagar</b>', confidence: 0.85 } }),
    })
    expect(screen.getByText('<b>pode apagar</b>')).toBeInTheDocument()
    expect(screen.getByText('Confiança: 85%')).toBeInTheDocument()
  })

  it('lists files on demand, capped at 200', async () => {
    const files = Array.from({ length: 250 }, (_, i) => ({
      name: `f${i}.bin`, path: `C:\\data\\f${i}.bin`, size: 10, modTime: 0, verdict: 'unknown', ruleId: '',
    }))
    m.ListFiles.mockResolvedValue(files)
    render(DetailsPanel, { node: node({ id: 3 }) })
    await userEvent.click(screen.getByRole('button', { name: strings.results.showFiles }))
    expect(await screen.findByText('f0.bin')).toBeInTheDocument()
    expect(screen.queryByText('f200.bin')).toBeNull()
    expect(screen.getByText(strings.results.filesLimit(200, 250))).toBeInTheDocument()
    expect(m.ListFiles).toHaveBeenCalledWith('C:\\', 3)
  })
})
