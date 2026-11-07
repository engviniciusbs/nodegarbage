import { describe, it, expect, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import Sidebar from './Sidebar.svelte'
import { route } from '$lib/route.svelte'
import { strings } from '$lib/strings'

describe('Sidebar', () => {
  beforeEach(() => {
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
