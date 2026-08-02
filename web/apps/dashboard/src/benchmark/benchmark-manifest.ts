import yuanhuManifestJson from './manifest.json';

export type BenchmarkPageLevel = 'P0' | 'P1' | 'P2';
export type BenchmarkImplementation = 'placeholder' | 'demo' | 'legacy-adapter' | 'native';
export type BenchmarkBackend = 'missing' | 'partial' | 'ready';
export type BenchmarkAcceptance = 'not-started' | 'unit-passed' | 'integration-passed' | 'e2e-passed';
export type BenchmarkRisk = 'low' | 'medium' | 'high';

export type BenchmarkEvidence = {
  spec: string;
  acceptance: string;
};

export type BenchmarkPage = {
  path: string;
  title: string;
  groupId: string | null;
  level: BenchmarkPageLevel;
  status?: string;
  screenshotVersion?: string;
  implementation?: BenchmarkImplementation;
  backend?: BenchmarkBackend;
  acceptance?: BenchmarkAcceptance;
  phase?: `3.${1 | 2 | 3 | 4 | 5 | 6}`;
  owner?: string;
  risk?: BenchmarkRisk;
  legacyRoutes?: readonly string[];
  evidence?: BenchmarkEvidence;
};

export type BenchmarkManifest = {
  groups: readonly { id: string; title: string }[];
  pages: readonly BenchmarkPage[];
};

export const benchmarkManifest = yuanhuManifestJson as BenchmarkManifest;
