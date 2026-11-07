import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent, within } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'

const handlers: Record<string, (d: unknown) => void> = {}
vi.mock('$lib/api', () => ({
  PlanClean: vi.fn(),
  ExecuteClean: vi.fn(),
  CancelClean: vi.fn(),
  on: vi.fn((ev: string, cb: (d: unknown) => void) => {
    handlers[ev] = cb
    return () => delete handlers[ev]
  }),
}))
vi.mock('$lib/stores/results.svelte', () => ({ refresh: vi.fn() }))

import * as api from '$lib/api'
import { refresh } from '$lib/stores/results.svelte'
import CleanDialog from './CleanDialog.svelte'
import { selection, toggle, clear } from '$lib/stores/selection.svelte'
import { startClean, clean } from '$lib/stores/clean.svelte'
import { strings } from '$lib/strings'
import { node } from '../../test/fixtures'

const m = vi.mocked(api, true)

function plan(over: Record<string, unknown> = {}) {
  return {
    id: 'p1',
    mode: 'recycle',
    token: 't1',
    items: [{ path: 'C:\\data\\item-1', size: 2 * 1024 * 1024, verdict: 'safe', category: 'dev-cache', source: 'rule', ruleId: 'r1', action: 'delete', warnings: [] }],
    refused: [],
    totalBytes: 2 * 1024 * 1024,
    expiresAt: '',
    ...over,
  }
}
const cautionItem = { path: 'C:\\data\\item-2', size: 1024, verdict: 'caution', category: 'user-data', source: 'rule', ruleId: 'r2', action: 'delete', warnings: [] }

let onClose: () => void
beforeEach(() => {
  vi.clearAllMocks()
  for (const k of Object.keys(handlers)) delete handlers[k]
  clear()
  clean.phase = 'planning'
  toggle(node({ id: 1 }))
  m.PlanClean.mockResolvedValue(plan() as never)
  m.ExecuteClean.mockResolvedValue(undefined as never)
  onClose = vi.fn()
})

const confirmBtn = () => screen.getByRole('button', { name: /^Limpar \d/ })

describe('CleanDialog', () => {
  it('plans in recycle mode on open and focuses the mode selector', async () => {
    render(CleanDialog, { open: true, onClose })
    await waitFor(() => expect(m.PlanClean).toHaveBeenCalledWith(['C:\\data\\item-1'], 'recycle'))
    expect(screen.getByRole('dialog')).toHaveAccessibleName(strings.clean.title)
    await waitFor(() => expect(confirmBtn()).toBeEnabled())
    expect(screen.getByRole('radio', { name: strings.clean.recycle })).toBeChecked()
    expect(document.activeElement).toBe(screen.getByRole('radio', { name: strings.clean.recycle }))
  })

  it('requires a per-item risk checkbox for caution items', async () => {
    m.PlanClean.mockResolvedValue(plan({ items: [plan().items[0], cautionItem], totalBytes: 3000 }) as never)
    render(CleanDialog, { open: true, onClose })
    const box = await screen.findByRole('checkbox', { name: strings.clean.understandRiskFor('item-2') })
    expect(confirmBtn()).toBeDisabled()
    await userEvent.click(box)
    expect(confirmBtn()).toBeEnabled()
  })

  it('permanent mode re-plans and needs its own confirmation', async () => {
    render(CleanDialog, { open: true, onClose })
    await waitFor(() => expect(confirmBtn()).toBeEnabled())
    m.PlanClean.mockResolvedValue(plan({ mode: 'permanent', token: 't2' }) as never)
    await userEvent.click(screen.getByRole('radio', { name: strings.clean.permanent }))
    await waitFor(() => expect(m.PlanClean).toHaveBeenLastCalledWith(['C:\\data\\item-1'], 'permanent'))
    const box = await screen.findByRole('checkbox', { name: strings.clean.understandPermanent })
    expect(confirmBtn()).toBeDisabled()
    await userEvent.click(box)
    expect(confirmBtn()).toBeEnabled()
  })

  it('shows translated refusals and suggests permanent mode', async () => {
    m.PlanClean.mockResolvedValue(plan({ refused: [{ path: 'C:\\big', reason: 'exceeds_recycle_bin' }, { path: 'C:\\x', reason: 'weird' }] }) as never)
    render(CleanDialog, { open: true, onClose })
    expect(await screen.findByText(strings.refusal.exceeds_recycle_bin)).toBeInTheDocument()
    expect(screen.getByText(strings.clean.suggestPermanent)).toBeInTheDocument()
    expect(screen.getByText(strings.refusal.unknown('weird'))).toBeInTheDocument()
  })

  it('disables confirm when nothing is accepted', async () => {
    m.PlanClean.mockResolvedValue(plan({ items: [], totalBytes: 0, refused: [{ path: 'C:\\x', reason: 'keep' }] }) as never)
    render(CleanDialog, { open: true, onClose })
    await screen.findByText(strings.refusal.keep)
    expect(screen.getByRole('button', { name: /^Limpar/ })).toBeDisabled()
  })

  it('executes exactly once, shows progress, and refreshes after done', async () => {
    render(CleanDialog, { open: true, onClose })
    await waitFor(() => expect(confirmBtn()).toBeEnabled())
    await userEvent.dblClick(confirmBtn())
    await waitFor(() => expect(m.ExecuteClean).toHaveBeenCalledTimes(1))
    expect(m.ExecuteClean).toHaveBeenCalledWith('p1', 't1')

    handlers['clean:progress']({ index: 0, total: 2, result: { path: 'C:\\data\\item-1', status: 'ok', reason: '', freed: 1 } })
    expect(await screen.findByRole('progressbar', { name: strings.clean.progressLabel })).toBeInTheDocument()
    expect(screen.getByText('C:\\data\\item-1')).toBeInTheDocument()

    handlers['clean:done']({
      freed: 2 * 1024 * 1024,
      results: [
        { path: 'a', status: 'ok', reason: '', freed: 1, skippedFiles: [] },
        { path: 'C:\\locked', status: 'skipped', reason: 'locked', freed: 0, skippedFiles: [] },
        { path: 'C:\\bad', status: 'error', reason: 'failed', freed: 0, skippedFiles: [] },
      ],
    })
    expect(await screen.findByText(strings.clean.doneTitle)).toBeInTheDocument()
    expect(screen.getByText(strings.refusal.locked)).toBeInTheDocument()
    expect(screen.getByText(strings.clean.okCount(1))).toBeInTheDocument()
    expect(screen.getByText(strings.clean.errorCount(1))).toBeInTheDocument()
    expect(selection.paths.size).toBe(0)
    expect(refresh).toHaveBeenCalled()
    await userEvent.click(screen.getByRole('button', { name: strings.clean.close }))
    expect(onClose).toHaveBeenCalled()
  })

  it('Esc closes before executing but not while executing', async () => {
    render(CleanDialog, { open: true, onClose })
    await waitFor(() => expect(confirmBtn()).toBeEnabled())
    const dlg = screen.getByRole('dialog')
    const ev1 = new Event('cancel', { cancelable: true })
    dlg.dispatchEvent(ev1)
    expect(onClose).toHaveBeenCalledTimes(1)
    expect(ev1.defaultPrevented).toBe(true)

    await userEvent.click(confirmBtn())
    await waitFor(() => expect(m.ExecuteClean).toHaveBeenCalled())
    const ev2 = new Event('cancel', { cancelable: true })
    await fireEvent(dlg, ev2)
    expect(ev2.defaultPrevented).toBe(true)
    expect(onClose).toHaveBeenCalledTimes(1)
    expect(screen.queryByRole('button', { name: strings.clean.close })).toBeNull()
    await userEvent.click(screen.getByRole('button', { name: strings.clean.cancelRun }))
    expect(m.CancelClean).toHaveBeenCalled()
  })

  it('offers a new plan when the token expired', async () => {
    m.ExecuteClean.mockRejectedValue('token expired')
    render(CleanDialog, { open: true, onClose })
    await waitFor(() => expect(confirmBtn()).toBeEnabled())
    await userEvent.click(confirmBtn())
    const alert = await screen.findByRole('alert')
    expect(within(alert).getByText(strings.clean.expired)).toBeInTheDocument()
    m.PlanClean.mockResolvedValue(plan({ id: 'p2', token: 't9' }) as never)
    m.ExecuteClean.mockResolvedValue(undefined as never)
    await userEvent.click(screen.getByRole('button', { name: strings.clean.newPlan }))
    await waitFor(() => expect(m.PlanClean).toHaveBeenCalledTimes(2))
    await waitFor(() => expect(confirmBtn()).toBeEnabled())
    await userEvent.click(confirmBtn())
    await waitFor(() => expect(m.ExecuteClean).toHaveBeenLastCalledWith('p2', 't9'))
    expect(m.ExecuteClean).toHaveBeenCalledTimes(2)
  })

  it('caps the accepted list at 100 rows with a count', async () => {
    const items = Array.from({ length: 130 }, (_, i) => ({ ...plan().items[0], path: `C:\\d\\f${i}` }))
    m.PlanClean.mockResolvedValue(plan({ items, totalBytes: 130 * 2 * 1024 * 1024 }) as never)
    render(CleanDialog, { open: true, onClose })
    expect(await screen.findByText(strings.clean.more(30))).toBeInTheDocument()
    expect(confirmBtn()).toHaveTextContent('260')
  })
})

describe('CleanDialog during execution', () => {
  it('re-opens after a native close, keeps onClose uncalled, and focuses Cancelar limpeza', async () => {
    render(CleanDialog, { open: true, onClose })
    await waitFor(() => expect(confirmBtn()).toBeEnabled())
    await userEvent.click(confirmBtn())
    await waitFor(() => expect(m.ExecuteClean).toHaveBeenCalled())
    const cancel = await screen.findByRole('button', { name: strings.clean.cancelRun })
    await waitFor(() => expect(cancel).toHaveFocus())
    const dlg = screen.getByRole('dialog') as HTMLDialogElement
    dlg.close()
    expect(dlg.open).toBe(true)
    expect(onClose).not.toHaveBeenCalled()
    expect(screen.getByRole('progressbar', { name: strings.clean.progressLabel })).toBeInTheDocument()
  })

  it('startClean during executing is a no-op', async () => {
    render(CleanDialog, { open: true, onClose })
    await waitFor(() => expect(confirmBtn()).toBeEnabled())
    await userEvent.click(confirmBtn())
    await waitFor(() => expect(m.ExecuteClean).toHaveBeenCalled())
    const calls = m.PlanClean.mock.calls.length
    await startClean(['C:\other'])
    expect(m.PlanClean.mock.calls.length).toBe(calls)
    expect(Object.keys(handlers)).toContain('clean:done')
  })

  it('focuses Gerar novo plano when an error returns to review', async () => {
    m.ExecuteClean.mockRejectedValue('token expired')
    render(CleanDialog, { open: true, onClose })
    await waitFor(() => expect(confirmBtn()).toBeEnabled())
    await userEvent.click(confirmBtn())
    const b = await screen.findByRole('button', { name: strings.clean.newPlan })
    await waitFor(() => expect(b).toHaveFocus())
  })
})
