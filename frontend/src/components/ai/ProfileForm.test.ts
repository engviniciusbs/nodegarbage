import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'

vi.mock('$lib/api', () => ({ SaveAIProfile: vi.fn(), TestAIProfile: vi.fn() }))

import * as api from '$lib/api'
import ProfileForm from './ProfileForm.svelte'

const m = vi.mocked(api, true)
const existing = { id: 'p1', name: 'Meu', kind: 'openai-compat', baseUrl: 'https://api.openai.com/v1', model: 'gpt-x', hasKey: true }

beforeEach(() => {
  vi.clearAllMocks()
  m.SaveAIProfile.mockImplementation(async (p) => ({ ...p, id: p.id || 'new', hasKey: true }) as never)
})

describe('ProfileForm', () => {
  it('prefills the OpenAI base URL and hides it for Anthropic', async () => {
    render(ProfileForm, { onSaved: vi.fn() })
    expect(screen.getByLabelText('URL base')).toHaveValue('https://api.openai.com/v1')
    expect(screen.getByLabelText('URL base')).toHaveAttribute('type', 'url')
    await userEvent.selectOptions(screen.getByLabelText('Tipo'), 'anthropic')
    expect(screen.queryByLabelText('URL base')).not.toBeInTheDocument()
  })

  it('sends an empty apiKey when the key is left unchanged', async () => {
    const onSaved = vi.fn()
    render(ProfileForm, { profile: existing as never, onSaved })
    expect(screen.getByLabelText('Chave de API')).toHaveAttribute('placeholder', '•••• (mantida)')
    expect(screen.getByLabelText('Chave de API')).toHaveAttribute('type', 'password')
    await userEvent.click(screen.getByRole('button', { name: 'Salvar perfil' }))
    await waitFor(() => expect(m.SaveAIProfile).toHaveBeenCalled())
    expect(m.SaveAIProfile.mock.calls[0][1]).toBe('')
    expect(m.SaveAIProfile.mock.calls[0][0]).toMatchObject({ id: 'p1', model: 'gpt-x' })
    expect(onSaved).toHaveBeenCalled()
  })

  it('validates required fields inline without saving', async () => {
    render(ProfileForm, { onSaved: vi.fn() })
    await userEvent.click(screen.getByRole('button', { name: 'Salvar perfil' }))
    expect(screen.getByText('Informe um nome.')).toBeInTheDocument()
    expect(screen.getByText('Informe o modelo.')).toBeInTheDocument()
    expect(m.SaveAIProfile).not.toHaveBeenCalled()
  })

  it('clears the key input after saving and never echoes it', async () => {
    render(ProfileForm, { profile: existing as never, onSaved: vi.fn() })
    const key = screen.getByLabelText('Chave de API')
    await userEvent.type(key, 'sk-secret')
    await userEvent.click(screen.getByRole('button', { name: 'Salvar perfil' }))
    await waitFor(() => expect(m.SaveAIProfile).toHaveBeenCalled())
    expect(m.SaveAIProfile.mock.calls[0][1]).toBe('sk-secret')
    await waitFor(() => expect(key).toHaveValue(''))
    expect(document.body.textContent).not.toContain('sk-secret')
  })

  it('shows success or the error message on "Testar conexão"', async () => {
    m.TestAIProfile.mockResolvedValueOnce(undefined as never)
    render(ProfileForm, { profile: existing as never, onSaved: vi.fn() })
    await userEvent.click(screen.getByRole('button', { name: 'Testar conexão' }))
    expect(await screen.findByText('Conexão OK')).toBeInTheDocument()
    m.TestAIProfile.mockRejectedValueOnce(new Error('Chave de API inválida'))
    await userEvent.click(screen.getByRole('button', { name: 'Testar conexão' }))
    expect(await screen.findByText('Chave de API inválida')).toBeInTheDocument()
    expect(screen.queryByText('Conexão OK')).not.toBeInTheDocument()
  })
})
