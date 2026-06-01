// Client-side validation shared by the forms so users get immediate feedback
// instead of a round-trip API error. Mirrors the backend rules in
// internal/config/validate.go (ValidateName) and the host:port parsing the
// tunnel form relies on.

export const MAX_NAME_LENGTH = 64

// validateName returns an error message, or null when the name is acceptable.
// `t` is the translation function so messages stay localized.
export function validateName(
  name: string,
  t: (key: string, params?: Record<string, string | number>) => string,
): string | null {
  if (!name) return t('validation.nameRequired')
  if (name.trim() !== name) return t('validation.nameWhitespace')
  if ([...name].length > MAX_NAME_LENGTH) return t('validation.nameTooLong', { max: MAX_NAME_LENGTH })
  for (const ch of name) {
    if (ch === '/' || ch === '\\') return t('validation.nameSlash')
    // Control characters (incl. newline/tab).
    if (ch.charCodeAt(0) < 0x20 || ch.charCodeAt(0) === 0x7f) return t('validation.nameControl')
  }
  return null
}

export interface Endpoint {
  host: string
  port: number
}

// parseEndpoint accepts "host:port" (and "[ipv6]:port"); returns null on
// malformed input so callers can show a precise message.
export function parseEndpoint(value: string): Endpoint | null {
  const raw = value.trim()
  if (!raw) return null

  let host: string
  let portStr: string
  if (raw.startsWith('[')) {
    const close = raw.indexOf(']')
    if (close < 0 || raw[close + 1] !== ':') return null
    host = raw.slice(1, close)
    portStr = raw.slice(close + 2)
  } else {
    const idx = raw.lastIndexOf(':')
    if (idx <= 0 || idx === raw.length - 1) return null
    host = raw.slice(0, idx)
    portStr = raw.slice(idx + 1)
  }

  if (!host) return null
  if (!/^\d+$/.test(portStr)) return null
  const port = Number(portStr)
  if (!Number.isInteger(port) || port < 1 || port > 65535) return null
  return { host, port }
}

export function formatEndpoint(host: string, port: number): string {
  if (!host && !port) return ''
  const h = host.includes(':') && !host.startsWith('[') ? `[${host}]` : host
  return `${h}:${port || ''}`
}
