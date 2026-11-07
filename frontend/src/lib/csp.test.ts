import { describe, it, expect } from 'vitest'
import html from '../../index.html?raw'

function csp(): Record<string, string[]> {
  const m = html.match(/<meta\s+http-equiv="Content-Security-Policy"\s+content="([^"]+)"/i)
  expect(m, 'CSP meta tag missing').not.toBeNull()
  const out: Record<string, string[]> = {}
  for (const d of m![1].split(';')) {
    const [name, ...vals] = d.trim().split(/\s+/)
    if (name) out[name] = vals
  }
  return out
}

describe('index.html CSP', () => {
  it('locks scripts to the app origin', () => {
    const p = csp()
    expect(p['default-src']).toEqual(["'self'"])
    expect(p['script-src']).toEqual(["'self'"])
    expect(p['img-src']).toEqual(["'self'", 'data:'])
    expect(p['object-src']).toEqual(["'none'"])
    expect(p['connect-src'][0]).toBe("'self'")
    expect(html).not.toMatch(/unsafe-eval/)
  })
})
