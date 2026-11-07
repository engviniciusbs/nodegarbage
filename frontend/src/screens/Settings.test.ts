import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'

vi.mock('$lib/api', () => ({
  GetSettings: vi.fn(),
  SaveSettings: vi.fn(),
  ListAIProfiles: vi.fn(),
  SaveAIProfile: vi.fn(),
  DeleteAIProfile: vi.fn(),
  TestAIProfile: vi.fn(),
}))

import * as api from '$lib/api'
import Settings from './Settings.svelte'
import { strings } from '$lib/strings'

const m = vi.mocked(api, true)
const base = {
  redactTerms: ['acme'],
  extraProtected: [],
  aiMinBytes: 500 * 1024 * 1024,
  workers: 8,
  userRulesDir: 'C:\\Users\\u\\AppData\\Local\\NodeGarbage\\rules.d',
  ruleProblems: [{ file: 'meu.yaml', ruleId: 'r9', error: 'campo inválido' }],
}

beforeEach(() => {
  vi.clearAllMocks()
  m.GetSettings.mockResolvedValue(structuredClone(base) as never)
  m.SaveSettings.mockResolvedValue(undefined as never)
  m.ListAIProfiles.mockResolvedValue([{ id: 'a', name: 'Meu GPT', kind: 'openai-compat', baseUrl: 'https://x/v1', model: 'gpt', hasKey: true }] as never)
})

describe('Settings', () => {
  it('lists AI profiles and offers the profile form', async () => {
    render(Settings)
    expect(await screen.findByText('Meu GPT')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: strings.profile.add }))
    expect(screen.getByLabelText(strings.profile.model)).toBeInTheDocument()
  })

  it('adds a redact term and a protected path and saves them', async () => {
    render(Settings)
    await userEvent.type(await screen.findByLabelText(strings.settings.termLabel), 'segredo')
    await userEvent.click(screen.getByRole('button', { name: strings.settings.termAdd }))
    await userEvent.type(screen.getByLabelText(strings.settings.pathLabel), 'D:\\Fotos{Enter}')
    expect(screen.getByText('D:\\Fotos')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: strings.settings.save }))
    await waitFor(() => expect(m.SaveSettings).toHaveBeenCalledTimes(1))
    expect(m.SaveSettings.mock.calls[0][0]).toMatchObject({
      redactTerms: ['acme', 'segredo'],
      extraProtected: ['D:\\Fotos'],
      aiMinBytes: 500 * 1024 * 1024,
      workers: 8,
    })
    expect(await screen.findByText(strings.settings.saved)).toBeInTheDocument()
  })

  it('removes a term', async () => {
    render(Settings)
    await userEvent.click(await screen.findByRole('button', { name: strings.settings.remove('acme') }))
    await userEvent.click(screen.getByRole('button', { name: strings.settings.save }))
    await waitFor(() => expect(m.SaveSettings.mock.calls[0][0].redactTerms).toEqual([]))
  })

  it('shows rule problems and a read-only rules dir', async () => {
    render(Settings)
    expect(await screen.findByText(/meu\.yaml: campo inválido/)).toBeInTheDocument()
    const dir = screen.getByLabelText(new RegExp(strings.settings.userRulesDir))
    expect(dir).toHaveAttribute('readonly')
    expect(screen.getByText(strings.settings.restartNote)).toBeInTheDocument()
  })

  it('shows an inline error when saving fails', async () => {
    m.SaveSettings.mockRejectedValue('falhou')
    render(Settings)
    await userEvent.click(await screen.findByRole('button', { name: strings.settings.save }))
    expect(await screen.findByRole('alert')).toHaveTextContent('falhou')
  })
})
