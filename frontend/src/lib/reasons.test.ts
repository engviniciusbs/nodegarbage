import { describe, it, expect } from 'vitest'
import { refusalText } from './reasons'
import { strings } from './strings'

describe('refusalText', () => {
  it('translates the link refusal', () => {
    expect(refusalText('link')).not.toBe(strings.refusal.unknown('link'))
  })
})
