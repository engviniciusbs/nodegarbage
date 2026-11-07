import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'

const handlers: Record<string, (d: unknown) => void> = {}
vi.mock('$lib/api', () => ({
  ListAIProfiles: vi.fn(),
  GetAIReport: vi.fn(),
  PreviewAIPayload: vi.fn(),
  RunAI: vi.fn(),
  CancelAI: vi.fn(),
  LocatePath: vi.fn(),
  GetChildren: vi.fn(),
  on: (ev: string, cb: (d: unknown) => void) => {
    handlers[ev] = cb
    return () => {}
  },
}))

import * as api from '$lib/api'
import AIReport from './AIReport.svelte'
import { route } from '$lib/route.svelte'
import { results } from '$lib/stores/results.svelte'
import { ai } from '$lib/stores/ai.svelte'
import { node } from '../test/fixtures'

const m = vi.mocked(api, true)
const profile = { id: 'p1', name: 'Meu', kind: 'openai-compat', baseUrl: '', model: 'gpt-x', hasKey: true }
const report = {
  markdown: '# Resumo\n\n**negrito** <script>alert(1)</script>',
  items: [{ path: 'C:\\data\\item-2', verdict: 'safe', reason: '<b>cache</b>', confidence: 0.9 }],
  createdAt: '2026-11-07T10:00:00Z',
}

beforeEach(() => {
  vi.clearAllMocks()
  ai.report = null
  ai.preview = null
  ai.error = ''
  ai.running = false
  ai.revealErrors = {}
  route.current = 'ai'
  m.ListAIProfiles.mockResolvedValue([profile] as never)
  m.GetAIReport.mockResolvedValue(null as never)
})

describe('AIReport screen', () => {
  it('shows an empty state linking to settings when there are no profiles', async () => {
    m.ListAIProfiles.mockResolvedValue([] as never)
    render(AIReport)
    await userEvent.click(await screen.findByRole('button', { name: 'Abrir Configurações' }))
    expect(route.current).toBe('settings')
  })

  it('previews the payload in a <pre> with tokens, provider/model and cache badge', async () => {
    m.PreviewAIPayload.mockResolvedValue({ json: '{"a":1}', tokens: 1234, provider: 'openai-compat', model: 'gpt-x', cached: true } as never)
    render(AIReport)
    await userEvent.click(await screen.findByRole('button', { name: 'Ver o que será enviado' }))
    const pre = await screen.findByText('{"a":1}')
    expect(pre.tagName).toBe('PRE')
    expect(screen.getByText(/tokens/)).toBeInTheDocument()
    expect(screen.getByText('openai-compat · gpt-x')).toBeInTheDocument()
    expect(screen.getByText('em cache')).toBeInTheDocument()
  })

  it('runs, disables until ai:done and renders the sanitized report', async () => {
    m.RunAI.mockResolvedValue(undefined as never)
    render(AIReport)
    await userEvent.click(await screen.findByRole('button', { name: 'Enviar para análise' }))
    expect(m.RunAI).toHaveBeenCalledWith('p1', false)
    expect(screen.getByRole('button', { name: 'Analisando…' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Cancelar' })).toBeInTheDocument()
    handlers['ai:done']({ report, error: '' })
    const strong = await screen.findByText('negrito')
    expect(strong.tagName).toBe('STRONG')
    expect(document.querySelector('script')).toBeNull()
    expect(screen.getByRole('button', { name: 'Enviar para análise' })).toBeEnabled()
    // reason is plain text, never HTML
    expect(screen.getByText('<b>cache</b>')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Reanalisar' }))
    expect(m.RunAI).toHaveBeenLastCalledWith('p1', true)
  })

  it('shows the last report on mount', async () => {
    m.GetAIReport.mockResolvedValue(report as never)
    render(AIReport)
    expect(await screen.findByText('negrito')).toBeInTheDocument()
  })

  it('shows ErrorState when ai:done carries an error', async () => {
    m.RunAI.mockResolvedValue(undefined as never)
    render(AIReport)
    await userEvent.click(await screen.findByRole('button', { name: 'Enviar para análise' }))
    handlers['ai:done']({ report: null, error: 'Falha de rede' })
    const alert = await screen.findByRole('alert')
    expect(within(alert).getByText('Falha de rede')).toBeInTheDocument()
  })

  it('reveals a cited path in Results and opens its parent', async () => {
    m.GetAIReport.mockResolvedValue(report as never)
    const parent = node({ id: 5, name: 'data', path: 'C:\\data' })
    const target = node({ id: 6, name: 'item-2', path: 'C:\\data\\item-2' })
    m.LocatePath.mockResolvedValue({ volume: 'C:\\', chain: [parent, target] } as never)
    m.GetChildren.mockResolvedValue([target] as never)
    render(AIReport)
    await userEvent.click(await screen.findByRole('button', { name: /Mostrar C:\\data\\item-2/ }))
    await waitFor(() => expect(route.current).toBe('results'))
    expect(route.volume).toBe('C:\\')
    expect(m.GetChildren).toHaveBeenCalledWith('C:\\', 5)
    expect(results.path.map((n) => n.id)).toEqual([5])
    expect(results.focusRequest).toBe(6)
  })

  it('shows an inline error when the path cannot be located', async () => {
    m.GetAIReport.mockResolvedValue(report as never)
    m.LocatePath.mockRejectedValue(new Error('item não encontrado na última varredura'))
    render(AIReport)
    await userEvent.click(await screen.findByRole('button', { name: /Mostrar/ }))
    expect(await screen.findByText('item não encontrado na última varredura')).toBeInTheDocument()
    expect(route.current).toBe('ai')
  })
})
