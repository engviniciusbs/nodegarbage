import { describe, it, expect } from 'vitest'
import { renderMarkdown } from './markdown'

describe('renderMarkdown', () => {
  it('removes script tags', () => {
    const html = renderMarkdown('oi <script>alert(1)</script> fim')
    expect(html).not.toMatch(/<script/i)
    expect(html).not.toContain('alert(1)')
  })
  it('removes img and event handler attributes', () => {
    const html = renderMarkdown('<img src=x onerror=alert(1)>\n\n<b onclick="x()">b</b>')
    expect(html).not.toMatch(/<img/i)
    expect(html).not.toMatch(/onerror|onclick/i)
  })
  it('removes iframe and style', () => {
    const html = renderMarkdown('<iframe src="https://x"></iframe><style>a{}</style>texto')
    expect(html).not.toMatch(/<iframe|<style/i)
    expect(html).toContain('texto')
  })
  it('never keeps a javascript: href', () => {
    const html = renderMarkdown('[x](javascript:alert(1))')
    expect(html).not.toMatch(/javascript:/i)
    expect(html).not.toMatch(/href/i)
  })
  it('renders links as plain text (no anchor, no navigation)', () => {
    const html = renderMarkdown('veja [docs](https://ok.example) e <a href="https://ok">raw</a>')
    expect(html).not.toMatch(/<a[\s>]/i)
    expect(html).toContain('docs')
    expect(html).toContain('raw')
  })
  it('renders bold and keeps line breaks', () => {
    const html = renderMarkdown('**negrito**\nlinha')
    expect(html).toContain('<strong>negrito</strong>')
    expect(html).toContain('<br')
  })
  it('renders lists, headings and code', () => {
    const html = renderMarkdown('# T\n\n- a\n- b\n\n`codigo`')
    expect(html).toMatch(/<h1>T<\/h1>/)
    expect(html).toContain('<li>a</li>')
    expect(html).toContain('<code>')
  })
})
