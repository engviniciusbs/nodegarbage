import { describe, it, expect } from 'vitest'
import type { AIItemDTO } from '$lib/api'
import { groupItems, groupOf, sumBytes, disabledReason, leafName, stricterVerdict, bulkSelectable } from './aireport'
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

  it('says when a cited path is a file or not a folder of the scan', () => {
    const miss = (missing: string) => item({ found: false, node: null as never, missing })
    expect(disabledReason(miss('file'))).toBe(strings.ai.missing.file)
    expect(disabledReason(miss('not_folder'))).toBe(strings.ai.missing.not_folder)
    expect(disabledReason(miss('not_found'))).toBe(strings.ai.notFound)
    expect(disabledReason(miss(''))).toBe(strings.ai.notFound)
  })

  it('reports the effective verdict only when it is stricter than the AI one', () => {
    expect(stricterVerdict(item({ verdict: 'safe', node: node({ verdict: 'caution' }) }))).toBe('caution')
    expect(stricterVerdict(item({ verdict: 'safe', node: node({ verdict: 'keep' }) }))).toBe('keep')
    expect(stricterVerdict(item({ verdict: 'caution', node: node({ verdict: 'caution' }) }))).toBe('')
    expect(stricterVerdict(item({ verdict: 'caution', node: node({ verdict: 'safe' }) }))).toBe('')
    expect(stricterVerdict(item({ found: false, node: null as never }))).toBe('')
  })

  it('bulk selection skips stricter-than-AI items; single selection does not', () => {
    const stricter = item({ verdict: 'safe', node: node({ verdict: 'caution' }) })
    expect(disabledReason(stricter)).toBe('')
    expect(bulkSelectable([item({}), stricter]).length).toBe(1)
  })

  it('takes the last path component', () => {
    expect(leafName('C:\\a\\b\\')).toBe('b')
    expect(leafName('C:\\')).toBe('C:')
  })
})
