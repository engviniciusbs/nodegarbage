import { describe, it, expect } from 'vitest'
import { visibleRange, ROW_HEIGHT } from './virtual'

describe('visibleRange', () => {
  it('starts at the top with overscan below', () => {
    expect(visibleRange(0, 400, 40, 1000)).toEqual({ start: 0, end: 18 })
  })
  it('applies overscan on both sides when scrolled', () => {
    expect(visibleRange(4000, 400, 40, 1000)).toEqual({ start: 92, end: 118 })
  })
  it('caps the end at count', () => {
    expect(visibleRange(0, 400, 40, 5)).toEqual({ start: 0, end: 5 })
    expect(visibleRange(39800, 400, 40, 1000).end).toBe(1000)
  })
  it('handles an empty list and exposes the row height', () => {
    expect(visibleRange(0, 400, 40, 0)).toEqual({ start: 0, end: 0 })
    expect(ROW_HEIGHT).toBe(40)
  })
})
