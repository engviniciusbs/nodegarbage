import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'

vi.mock('$lib/api', () => ({ IsElevated: vi.fn(), RestartElevated: vi.fn() }))

import * as api from '$lib/api'
import Sidebar from './Sidebar.svelte'
import { route } from '$lib/route.svelte'
import { strings } from '$lib/strings'

const m = vi.mocked(api, true)

describe('Sidebar', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    m.IsElevated.mockResolvedValue(true)
    route.current = 'home'
  })
  it('renders six nav items inside a labelled nav', () => {
    render(Sidebar)
    expect(screen.getByRole('navigation', { name: strings.nav.label })).toBeInTheDocument()
    expect(screen.getAllByRole('button')).toHaveLength(6)
  })
  it('navigates on click', async () => {
    render(Sidebar)
    await userEvent.click(screen.getByRole('button', { name: strings.nav.history }))
    expect(route.current).toBe('history')
    expect(screen.getByRole('button', { name: strings.nav.history })).toHaveAttribute('aria-current', 'page')
    expect(screen.getByRole('button', { name: strings.nav.home })).not.toHaveAttribute('aria-current')
  })
  it('marks the active item', () => {
    render(Sidebar)
    expect(screen.getByRole('button', { name: strings.nav.home })).toHaveAttribute('aria-current', 'page')
  })
})

describe('Sidebar admin notice', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    route.current = 'home'
  })
  it('shows the notice when not elevated and restarts on click', async () => {
    m.IsElevated.mockResolvedValue(false)
    m.RestartElevated.mockResolvedValue(undefined)
    render(Sidebar)
    const region = await screen.findByRole('region', { name: strings.home.adminTitle })
    expect(region).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: strings.home.adminRestart }))
    expect(m.RestartElevated).toHaveBeenCalled()
  })

  it('hides the notice when elevated', async () => {
    m.IsElevated.mockResolvedValue(true)
    render(Sidebar)
    await Promise.resolve()
    await Promise.resolve()
    expect(m.IsElevated).toHaveBeenCalled()
    expect(screen.queryByRole('button', { name: strings.home.adminRestart })).toBeNull()
  })

  it('shows an inline error when RestartElevated rejects', async () => {
    m.IsElevated.mockResolvedValue(false)
    m.RestartElevated.mockRejectedValue(new Error('uac negado'))
    render(Sidebar)
    await userEvent.click(await screen.findByRole('button', { name: strings.home.adminRestart }))
    expect(await screen.findByText(/uac negado/)).toBeInTheDocument()
  })
})
