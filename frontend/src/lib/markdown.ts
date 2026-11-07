import { marked } from 'marked'
import DOMPurify from 'dompurify'

// The AI report is untrusted text. Links, images and embedded content are
// dropped (text kept) so the WebView can never navigate or load remote content.
const ALLOWED_TAGS = [
  'p', 'br', 'hr', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'strong', 'b', 'em', 'i', 'del',
  'ul', 'ol', 'li', 'blockquote', 'pre', 'code', 'table', 'thead', 'tbody', 'tr', 'th', 'td',
]

/** Markdown → sanitized HTML, safe for {@html}. */
export function renderMarkdown(md: string): string {
  const raw = marked.parse(md ?? '', { async: false, breaks: true, gfm: true }) as string
  return DOMPurify.sanitize(raw, {
    ALLOWED_TAGS,
    ALLOWED_ATTR: [],
    ALLOWED_URI_REGEXP: /^https?:/i,
    KEEP_CONTENT: true,
  })
}
