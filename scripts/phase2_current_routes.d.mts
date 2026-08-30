export type Phase2App = 'dashboard' | 'sidebar' | 'operation';

export type Phase2Route = {
  path: string;
  target: string;
  auth?: boolean;
  corpContext?: boolean;
  permission?: string | null;
};

export type CurrentPhase2Routes = Record<Phase2App, Phase2Route[]>;

export const phase2Apps: readonly Phase2App[];
export function currentPhase2Routes(root?: string): CurrentPhase2Routes;
export function currentPhase2RouteCount(routes: CurrentPhase2Routes): number;
export function expectedPhase2PlaywrightTitles(routes: CurrentPhase2Routes): string[];
