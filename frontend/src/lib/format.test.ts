import { describe, it, expect } from 'vitest'
import { formatBytes, formatCount, formatDate, formatDuration, formatTime } from './format'

describe('format', () => {
  it('formatBytes', () => {
    expect(formatBytes(0)).toBe('0 B')
    expect(formatBytes(1536)).toBe('1,5 KB')
    expect(formatBytes(1610612736)).toBe('1,5 GB')
    expect(formatBytes(512)).toBe('512 B')
  })
  it('formatCount', () => {
    expect(formatCount(1234567)).toBe('1.234.567')
  })
  it('formatDate', () => {
    expect(formatDate(Date.UTC(2026, 10, 6, 12) / 1000)).toBe('06/11/2026')
  })
  it('formatTime', () => {
    expect(formatTime(new Date(2026, 10, 6, 9, 5).getTime() / 1000)).toBe('09:05')
  })
  it('formatDuration', () => {
    expect(formatDuration(45_000)).toBe('45 s')
    expect(formatDuration(161_000)).toBe('2 min 41 s')
    expect(formatDuration(120_000)).toBe('2 min')
    expect(formatDuration(3_780_000)).toBe('1 h 3 min')
  })
})
