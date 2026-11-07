import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/svelte'
import VerdictBadge from './VerdictBadge.svelte'
import { strings } from '$lib/strings'

describe('VerdictBadge', () => {
  it('shows label and decorative icon', () => {
    const { container } = render(VerdictBadge, { verdict: 'caution' })
    expect(screen.getByText(strings.verdict.caution)).toBeInTheDocument()
    expect(container.querySelector('svg')).toHaveAttribute('aria-hidden', 'true')
  })
  it('flags AI source', () => {
    render(VerdictBadge, { verdict: 'safe', source: 'ai' })
    expect(screen.getByText(/sugerido pela IA/)).toBeInTheDocument()
  })
  it('omits AI note for rule source', () => {
    render(VerdictBadge, { verdict: 'keep', source: 'rule' })
    expect(screen.queryByText(/sugerido pela IA/)).toBeNull()
  })
})
