export function operationBasename(pathname: string): string {
  return pathname === '/operation-app' || pathname.startsWith('/operation-app/')
    ? '/operation-app'
    : '/';
}
