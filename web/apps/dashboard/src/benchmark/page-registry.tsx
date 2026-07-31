import type { ReactNode } from 'react';

import type { BenchmarkManifest } from './benchmark-manifest';
import { PlaceholderPage } from './placeholder-page';

export type PageRegistry = Readonly<Record<string, ReactNode>>;

export function createPageRegistry({
  manifest,
  p0Pages,
  p1Pages,
}: {
  manifest: BenchmarkManifest;
  p0Pages: PageRegistry;
  p1Pages: PageRegistry;
}): PageRegistry {
  const groupTitles = new Map(manifest.groups.map((group) => [group.id, group.title]));

  return Object.fromEntries(manifest.pages.map((page) => [
    page.path,
    p0Pages[page.path]
      ?? p1Pages[page.path]
      ?? <PlaceholderPage
        title={page.title}
        {...(page.groupId === null
          ? {}
          : { groupTitle: groupTitles.get(page.groupId) ?? '工作台' })}
      />,
  ]));
}
