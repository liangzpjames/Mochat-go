const INTERNAL_ORIGIN = 'https://mobile.internal';

export function safeInternalTarget(
  raw: string | null | undefined,
  fallback: string,
): string {
  if (
    !raw
    || raw !== raw.trim()
    || !raw.startsWith('/')
    || raw.startsWith('//')
    || raw.includes('\\')
  ) {
    return fallback;
  }

  try {
    decodeURI(raw);
    const parsed = new URL(raw, INTERNAL_ORIGIN);
    if (parsed.origin !== INTERNAL_ORIGIN) {
      return fallback;
    }
    return raw;
  } catch {
    return fallback;
  }
}
