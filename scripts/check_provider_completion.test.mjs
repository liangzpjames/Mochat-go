import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';

import { checkProviderCompletion } from './check_provider_completion.mjs';

async function writeFixture(files) {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'provider-gate-'));
  for (const [relative, contents] of Object.entries(files)) {
    const target = path.join(root, relative);
    await fs.mkdir(path.dirname(target), { recursive: true });
    await fs.writeFile(target, contents, 'utf8');
  }
  return root;
}

test('fails when a real provider implementation has no classified registration', async () => {
  const root = await writeFixture({
    'internal/modules/providers/archive/wecom/archive.go': `package wecom
import "jiyi/mochat-go/internal/modules/providers"
func (a Archive) Status() providers.Status { return providers.Status{Kind: "wecom_archive", State: providers.StateLimited} }
`,
    'internal/modules/providers/catalog/catalog.go': `package catalog
// providers.Registration{Kind: "wecom_archive", Source: providers.SourceExternal}
`,
    'internal/modules/providers/catalog/unclassified.go': `package catalog
import "jiyi/mochat-go/internal/modules/providers"
func (p Unclassified) Status() providers.Status { return providers.Status{Kind: "unclassified", State: providers.StateLimited} }
`,
    'internal/modules/providers/archive/wecom/testdata/fixture.go': `package fixture
var registration = "providers.Registration{Kind: wecom_archive, Source: external}"
`,
  });
  const result = await checkProviderCompletion(root);
  assert.equal(result.ok, false);
  assert.match(result.errors.join('\n'), /wecom_archive/);
});

test('accepts a real source registration and rejects archive ready self-certification', async () => {
  const root = await writeFixture({
    'internal/modules/providers/archive/wecom/archive.go': `package wecom
import "jiyi/mochat-go/internal/modules/providers"
func (a Archive) Kind() providers.Source { return providers.SourceExternal }
func (a Archive) Status() providers.Status { return providers.Status{Kind: "wecom_archive", State: providers.StateLimited, Code: "archive.getchatdata_unimplemented"} }
`,
    'internal/modules/providers/catalog/catalog.go': `package catalog
import "jiyi/mochat-go/internal/modules/providers"
func NewRegistry() *providers.Registry {
  registry := providers.NewRegistry()
  registrations := []providers.Registration{{Kind: "wecom_archive", Source: providers.SourceExternal, Capabilities: []string{"archive"}}}
  for _, registration := range registrations { _ = registry.Register(registration) }
  return registry
}
`,
  });
  const result = await checkProviderCompletion(root);
  assert.equal(result.ok, true, result.errors.join('\n'));
});

test('fails when external archive status hides state or code behind helpers', async () => {
  const root = await writeFixture({
    'internal/modules/providers/archive/wecom/archive.go': `package wecom
import "jiyi/mochat-go/internal/modules/providers"
func hiddenState() providers.State { return providers.State("ready") }
func hiddenCode() string { return "archive.getchatdata_unimplemented" }
type ExternalSource struct{}
func (ExternalSource) Kind() providers.Source { return providers.SourceExternal }
func (ExternalSource) Status() providers.Status { return providers.Status{Kind: "wecom_archive", Source: providers.SourceExternal, State: hiddenState(), Code: hiddenCode()} }
`,
    'internal/modules/providers/catalog/catalog.go': `package catalog
import "jiyi/mochat-go/internal/modules/providers"
func NewRegistry() *providers.Registry {
  registry := providers.NewRegistry()
  registration := providers.Registration{Kind: "wecom_archive", Source: providers.SourceExternal}
  _ = registry.Register(registration)
  return registry
}
`,
  });
  const result = await checkProviderCompletion(root);
  assert.equal(result.ok, false);
  assert.match(result.errors.join('\n'), /external Status/);
});

test('fails when external archive Kind hides SourceExternal behind a helper', async () => {
  const root = await writeFixture({
    'internal/modules/providers/archive/wecom/archive.go': `package wecom
import "jiyi/mochat-go/internal/modules/providers"
func externalKind() providers.Source { return providers.SourceExternal }
type ExternalSource struct{}
func (ExternalSource) Kind() providers.Source { return externalKind() }
func (ExternalSource) Status() providers.Status { return providers.Status{Kind: "wecom_archive", Source: providers.SourceExternal, State: providers.StateLimited, Code: "archive.getchatdata_unimplemented"} }
`,
    'internal/modules/providers/catalog/catalog.go': `package catalog
import "jiyi/mochat-go/internal/modules/providers"
func NewRegistry() *providers.Registry {
  registry := providers.NewRegistry()
  registration := providers.Registration{Kind: "wecom_archive", Source: providers.SourceExternal}
  _ = registry.Register(registration)
  return registry
}
`,
  });
  const result = await checkProviderCompletion(root);
  assert.equal(result.ok, false);
  assert.match(result.errors.join('\n'), /Kind must directly return/);
});

test('fails when a dead registration literal is not on a Register call path', async () => {
  const root = await writeFixture({
    'internal/modules/providers/archive/wecom/archive.go': `package wecom
import "jiyi/mochat-go/internal/modules/providers"
func (a Archive) Status() providers.Status { return providers.Status{Kind: "wecom_archive", State: providers.StateLimited} }
`,
    'internal/modules/providers/catalog/catalog.go': `package catalog
import "jiyi/mochat-go/internal/modules/providers"
var unused = providers.Registration{Kind: "wecom_archive", Source: providers.SourceExternal}
func deadRegistry() {
  registry := providers.NewRegistry()
  registration := providers.Registration{Kind: "wecom_archive", Source: providers.SourceExternal}
  _ = registry.Register(registration)
}
`,
  });
  const result = await checkProviderCompletion(root);
  assert.equal(result.ok, false);
  assert.match(result.errors.join('\n'), /wecom_archive/);
});

test('fails when NewRegistry hides registration behind an unreachable branch', async () => {
  const root = await writeFixture({
    'internal/modules/providers/archive/wecom/archive.go': `package wecom
import "jiyi/mochat-go/internal/modules/providers"
func (a Archive) Status() providers.Status { return providers.Status{Kind: "wecom_archive", State: providers.StateLimited} }
`,
    'internal/modules/providers/catalog/catalog.go': `package catalog
import "jiyi/mochat-go/internal/modules/providers"
func NewRegistry() *providers.Registry {
  registry := providers.NewRegistry()
  if (false) {
    registration := providers.Registration{Kind: "wecom_archive", Source: providers.SourceExternal}
    _ = registry.Register(registration)
  }
  return registry
}
`,
  });
  const result = await checkProviderCompletion(root);
  assert.equal(result.ok, false);
  assert.match(result.errors.join('\n'), /unreachable|wecom_archive/);
});

test('fails when NewRegistry puts registration in an else branch', async () => {
  const root = await writeFixture({
    'internal/modules/providers/archive/wecom/archive.go': `package wecom
import "jiyi/mochat-go/internal/modules/providers"
func (a Archive) Status() providers.Status { return providers.Status{Kind: "wecom_archive", State: providers.StateLimited} }
`,
    'internal/modules/providers/catalog/catalog.go': `package catalog
import "jiyi/mochat-go/internal/modules/providers"
func NewRegistry() *providers.Registry {
  registry := providers.NewRegistry()
  if true { return registry } else {
    registration := providers.Registration{Kind: "wecom_archive", Source: providers.SourceExternal}
    _ = registry.Register(registration)
  }
  return registry
}
`,
  });
  const result = await checkProviderCompletion(root);
  assert.equal(result.ok, false);
  assert.match(result.errors.join('\n'), /unreachable|wecom_archive/);
});
