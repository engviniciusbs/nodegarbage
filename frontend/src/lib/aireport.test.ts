import { describe, it, expect } from 'vitest'
import type { AIItemDTO } from '$lib/api'
import { groupItems, groupOf, sumBytes, disabledReason, leafName } from './aireport'
import { strings } from './strings'
import { node } from '../test/fixtures'

const item = (over: Partial<AIItemDTO>): AIItemDTO =>
  ({ path: 'C:\\x', verdict: 'safe', reason: '', confidence: 1, found: true, selectable: true, node: node(), ...over }) as AIItemDTO

describe('aireport helpers', () => {
  it('groups by AI verdict; unexpected verdicts go to caution', () => {
    expect(groupOf('safe')).toBe('safe')
    expect(groupOf('keep')).toBe('keep')
    expect(groupOf('caution')).toBe('caution')
    expect(groupOf('weird')).toBe('caution')
    const g = groupItems([item({ verdict: 'keep' }), item({ verdict: 'safe' }), item({ verdict: '' })])
    expect([g.safe.length, g.caution.length, g.keep.length]).toEqual([1, 1, 1])
  })

  it('sums only items found in the scan', () => {
    expect(sumBytes([item({ node: node({ size: 5 }) }), item({ found: false, node: null as never })])).toBe(5)
  })

  it('explains why an item cannot be selected', () => {
    expect(disabledReason(item({}))).toBe('')
    expect(disabledReason(item({ found: false, node: null as never }))).toBe(strings.ai.notFound)
    expect(disabledReason(item({ node: node({ action: 'manual' }) }))).toBe(strings.results.blocked.manual)
    expect(disabledReason(item({ selectable: false }))).toBe(strings.ai.notSelectable)
  })

  it('takes the last path component', () => {
    expect(leafName('C:\\a\\b\\')).toBe('b')
    expect(leafName('C:\\')).toBe('C:')
  })
})
