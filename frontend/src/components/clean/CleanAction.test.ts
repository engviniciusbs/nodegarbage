import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'

vi.mock('$lib/api', () => ({ PlanClean: vi.fn(), ExecuteClean: vi.fn(), CancelClean: vi.fn(), on: vi.fn(() => () => {}) }))
vi.mock('$lib/stores/results.svelte', () => ({ refresh: vi.fn() }))

import * as api from '$lib/api'
import CleanAction from './CleanAction.svelte'
import { toggle, clear } from '$lib/stores/selection.svelte'
import { strings } from '$lib/strings'
import { node } from '../../test/fixtures'

beforeEach(() => {
  vi.clearAllMocks()
  clear()
})

describe('CleanAction', () => {
  it('is disabled with an empty selection and never plans', () => {
    render(CleanAction)
    expect(screen.getByRole('button', { name: strings.clean.button(0, '0 B') })).toBeDisabled()
    expect(api.PlanClean).not.toHaveBeenCalled()
  })

  it('opens the dialog and returns focus to the button on close', async () => {
    toggle(node({ id: 1 }))
    vi.mocked(api.PlanClean).mockResolvedValue({ id: 'p', mode: 'recycle', token: 't', items: [], refused: [], totalBytes: 0, expiresAt: '' } as never)
    render(CleanAction)
    const btn = screen.getByRole('button', { name: strings.clean.button(1, '1,0 MB') })
    await userEvent.click(btn)
    await waitFor(() => expect(api.PlanClean).toHaveBeenCalledWith(['C:\\data\\item-1'], 'recycle'))
    await userEvent.keyboard('{Tab}')
    await userEvent.click(await screen.findByRole('button', { name: strings.clean.cancel }))
    await waitFor(() => expect(btn).toHaveFocus())
  })
})

describe('CleanAction while cleaning', () => {
  it('is disabled while a cleaning is executing', async () => {
    const { clean } = await import('$lib/stores/clean.svelte')
    toggle(node({ id: 1 }))
    clean.phase = 'executing'
    render(CleanAction)
    expect(screen.getByRole('button', { name: strings.clean.button(1, '1,0 MB') })).toBeDisabled()
    clean.phase = 'planning'
  })
})
