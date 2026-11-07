import { strings } from './strings'

/** PT-BR text for a refusal/skip reason code; unknown codes fall back to a generic text with the raw code. */
export function refusalText(code: string): string {
  const t = strings.refusal as Record<string, unknown>
  const v = Object.prototype.hasOwnProperty.call(t, code) ? t[code] : undefined
  return typeof v === 'string' ? v : strings.refusal.unknown(code)
}

export function warningText(code: string): string {
  const t = strings.warning as Record<string, unknown>
  const v = Object.prototype.hasOwnProperty.call(t, code) ? t[code] : undefined
  return typeof v === 'string' ? v : strings.warning.unknown(code)
}

/** Last path segment, for accessible names. */
export function baseName(path: string): string {
  const parts = path.split(/[\\/]/).filter(Boolean)
  return parts[parts.length - 1] ?? path
}
