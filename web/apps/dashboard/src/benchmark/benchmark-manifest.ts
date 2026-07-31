import yuanhuManifestJson from './manifest.json';

export type BenchmarkPageLevel = 'P0' | 'P1' | 'P2';

export type BenchmarkPage = {
  path: string;
  title: string;
  groupId: string | null;
  level: BenchmarkPageLevel;
};

export type BenchmarkManifest = {
  groups: readonly { id: string; title: string }[];
  pages: readonly BenchmarkPage[];
};

export const benchmarkManifest = yuanhuManifestJson as BenchmarkManifest;
