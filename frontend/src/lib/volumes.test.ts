import { describe, it, expect } from 'vitest'
import { pickScannedVolume, scanTime } from './volumes'

describe('pickScannedVolume', () => {
  it('picks the most recently scanned volume', () => {
    expect(
      pickScannedVolume([
        { root: 'C:\\', lastScan: '2026-11-01T10:00:00Z' },
        { root: 'D:\\', lastScan: null },
        { root: 'F:\\', lastScan: '2026-11-05T10:00:00Z' },
      ]),
    ).toBe('F:\\')
  })
  it('ignores the Go zero time and returns empty when nothing was scanned', () => {
    expect(pickScannedVolume([{ root: 'C:\\', lastScan: '0001-01-01T00:00:00Z' }, { root: 'D:\\', lastScan: null }])).toBe('')
    expect(scanTime({ lastScan: undefined })).toBeNull()
  })
})
