export function updateSearch(
  current: URLSearchParams,
  changes: Record<string, string | number | undefined>,
): URLSearchParams {
  const next = new URLSearchParams(current);

  for (const [key, value] of Object.entries(changes)) {
    if (value === undefined || String(value).trim() === '') {
      next.delete(key);
    } else {
      next.set(key, String(value));
    }
  }

  return next;
}
