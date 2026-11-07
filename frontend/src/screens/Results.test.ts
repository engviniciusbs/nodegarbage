import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'

vi.mock('$lib/api', () => ({ GetChildren: vi.fn(), GetSummary: vi.fn(), ListFiles: vi.fn() }))

import * as api from '$lib/api'
import Results from './Results.svelte'
import { route } from '$lib/route.svelte'
import { results, loadVolume } from '$lib/stores/results.svelte'
import { selection, clear } from '$lib/stores/selection.svelte'
import { node } from '../test/fixtures'

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
})
