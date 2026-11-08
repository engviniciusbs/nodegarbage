import { describe, it, expect, beforeEach } from 'vitest'
import { markSolidBackdrop } from './platform'

const nav = (platform: string, platformVersion: string) =>
  ({ userAgentData: { getHighEntropyValues: async () => ({ platform, platformVersion }) } }) as unknown as Navigator

beforeEach(() => document.documentElement.classList.remove('solid'))

describe('markSolidBackdrop', () => {
  it('marks Windows 10 as solid', async () => {
    await markSolidBackdrop(document, nav('Windows', '10.0.0'))
    expect(document.documentElement).toHaveClass('solid')
  })
  it('keeps Mica on Windows 11', async () => {
    await markSolidBackdrop(document, nav('Windows', '15.0.0'))
    expect(document.documentElement).not.toHaveClass('solid')
  })
  it('does nothing without userAgentData', async () => {
    await markSolidBackdrop(document, {} as Navigator)
    expect(document.documentElement).not.toHaveClass('solid')
  })
})
