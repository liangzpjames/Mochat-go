export function sidebarBasename(pathname: string): string {
  return pathname === '/sidebar-app' || pathname.startsWith('/sidebar-app/')
    ? '/sidebar-app'
    : '/';
}
