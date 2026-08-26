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
  registration := providers.Registration{Kind: "wecom_archive", Source: providers.SourceExternal, Capabilities: []string{"archive"}}
  _ = registry.Register(registration)
  return registry
}
`,
  });
  const result = await checkProviderCompletion(root);
  assert.equal(result.ok, true, result.errors.join('\n'));
});

test('accepts bridge archive ready status only with a direct bridge fetch path', async () => {
  const root = await writeFixture({
    'internal/modules/providers/archive/bridge_source.go': `package archive
import "jiyi/mochat-go/internal/modules/providers"
type BridgeSource struct { client *BridgeArchiveClient }
type BridgeArchiveClient struct{}
func (BridgeSource) Kind() providers.Source { return providers.SourceExternal }
func (s BridgeSource) Status() providers.Status { return providers.Status{Kind: "wecom_archive", Source: providers.SourceExternal, State: providers.StateReady, Code: "archive.bridge_ready"} }
func (s BridgeSource) Fetch() { s.client.fetchMessages() }
func (BridgeArchiveClient) fetchMessages() {}
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

test('fails when NewRegistry hides registration behind a runtime or environment helper', async () => {
  const root = await writeFixture({
    'internal/modules/providers/archive/wecom/archive.go': `package wecom
import "jiyi/mochat-go/internal/modules/providers"
func (a Archive) Kind() providers.Source { return providers.SourceExternal }
func (a Archive) Status() providers.Status { return providers.Status{Kind: "wecom_archive", State: providers.StateLimited, Code: "archive.getchatdata_unimplemented"} }
`,
    'internal/modules/providers/catalog/catalog.go': `package catalog
import "jiyi/mochat-go/internal/modules/providers"
func runtimeFlagNeverEnabled() bool { return false }
func NewRegistry() *providers.Registry {
  registry := providers.NewRegistry()
  registration := providers.Registration{Kind: "wecom_archive", Source: providers.SourceExternal}
  if runtimeFlagNeverEnabled() {
    _ = registry.Register(registration)
  }
  return registry
}
`,
  });
  const result = await checkProviderCompletion(root);
  assert.equal(result.ok, false);
  assert.match(result.errors.join('\n'), /unconditional|unreachable|wecom_archive/);
});

test('rejects Register calls after continue or break inside a loop', async () => {
  const root = await writeFixture({
    'internal/modules/providers/archive/wecom/archive.go': `package wecom
import "jiyi/mochat-go/internal/modules/providers"
func (a Archive) Status() providers.Status { return providers.Status{Kind: "wecom_archive", State: providers.StateLimited} }
`,
    'internal/modules/providers/catalog/catalog.go': `package catalog
import "jiyi/mochat-go/internal/modules/providers"
func NewRegistry() *providers.Registry {
  registry := providers.NewRegistry()
  registration := providers.Registration{Kind: "wecom_archive", Source: providers.SourceExternal}
  for {
    continue
    _ = registry.Register(registration)
  }
  for {
    break
    _ = registry.Register(registration)
  }
  return registry
}
`,
  });
  const result = await checkProviderCompletion(root);
  assert.equal(result.ok, false);
  assert.match(result.errors.join('\n'), /top-level|unconditional|loop/);
});

test('rejects Register calls hidden inside a function literal', async () => {
  const root = await writeFixture({
    'internal/modules/providers/archive/wecom/archive.go': `package wecom
import "jiyi/mochat-go/internal/modules/providers"
func (a Archive) Status() providers.Status { return providers.Status{Kind: "wecom_archive", State: providers.StateLimited} }
`,
    'internal/modules/providers/catalog/catalog.go': `package catalog
import "jiyi/mochat-go/internal/modules/providers"
func NewRegistry() *providers.Registry {
  registry := providers.NewRegistry()
  registration := providers.Registration{Kind: "wecom_archive", Source: providers.SourceExternal}
  registerLater := func() { _ = registry.Register(registration) }
  _ = registerLater
  return registry
}
`,
  });
  const result = await checkProviderCompletion(root);
  assert.equal(result.ok, false);
  assert.match(result.errors.join('\n'), /top-level|function literal/);
});

test('fails when one package and receiver define duplicate Kind methods', async () => {
  const root = await writeFixture({
    'internal/modules/providers/archive/wecom/archive.go': `package wecom
import "jiyi/mochat-go/internal/modules/providers"
type ExternalSource struct{}
func (ExternalSource) Kind() providers.Source { return providers.SourceExternal }
func (ExternalSource) Status() providers.Status { return providers.Status{Kind: "wecom_archive", State: providers.StateLimited, Code: "archive.getchatdata_unimplemented"} }
`,
    'internal/modules/providers/archive/wecom/duplicate_kind.go': `package wecom
import "jiyi/mochat-go/internal/modules/providers"
func (ExternalSource) Kind() providers.Source { return providers.SourceExternal }
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
  assert.match(result.errors.join('\n'), /duplicate archive Kind/);
});

test('fails when one package and receiver define duplicate Status methods', async () => {
  const root = await writeFixture({
    'internal/modules/providers/archive/wecom/archive.go': `package wecom
import "jiyi/mochat-go/internal/modules/providers"
type ExternalSource struct{}
func (ExternalSource) Kind() providers.Source { return providers.SourceExternal }
func (ExternalSource) Status() providers.Status { return providers.Status{Kind: "wecom_archive", State: providers.StateLimited, Code: "archive.getchatdata_unimplemented"} }
`,
    'internal/modules/providers/archive/wecom/duplicate_status.go': `package wecom
import "jiyi/mochat-go/internal/modules/providers"
func (ExternalSource) Status() providers.Status { return providers.Status{Kind: "wecom_archive", State: providers.StateUnavailable, Code: "archive.getchatdata_unimplemented"} }
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
  assert.match(result.errors.join('\n'), /duplicate archive Status/);
});

test('accepts a generic receiver while binding Kind and Status to its concrete receiver', async () => {
  const root = await writeFixture({
    'internal/modules/providers/archive/wecom/archive.go': `package wecom
import "jiyi/mochat-go/internal/modules/providers"
type ExternalSource[T any] struct{}
func (ExternalSource[T]) Kind() providers.Source { return providers.SourceExternal }
func (ExternalSource[T]) Status() providers.Status { return providers.Status{Kind: "wecom_archive", State: providers.StateLimited, Code: "archive.getchatdata_unimplemented"} }
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
  assert.equal(result.ok, true, result.errors.join('\n'));
});

test('rejects ignored-only provider implementations in the runtime registration reverse contract', async () => {
  const root = await writeFixture({
    'internal/modules/providers/archive/wecom/ignored.go': `//go:build ignore
package wecom
import "jiyi/mochat-go/internal/modules/providers"
type ExternalSource struct{}
func (ExternalSource) Kind() providers.Source { return providers.SourceExternal }
func (ExternalSource) Status() providers.Status { return providers.Status{Kind: "wecom_archive", State: providers.StateReady, Code: "archive.fake_ready"} }
`,
    'internal/modules/providers/ignored/ignored.go': `//go:build ignore
package ignored
import "jiyi/mochat-go/internal/modules/providers"
func ignoredStatus() providers.Status { return providers.Status{Kind: "ignored_provider", State: providers.StateReady} }
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
  assert.match(result.errors.join('\n'), /active production implementation|wecom_archive/);
});

test('scans linux-only active implementations for the production image on a Windows host', async () => {
  const root = await writeFixture({
    'internal/modules/providers/archive/wecom/linux_only.go': `//go:build linux
package wecom
import "jiyi/mochat-go/internal/modules/providers"
type LinuxArchive struct{}
func (LinuxArchive) Kind() providers.Source { return providers.SourceExternal }
func (LinuxArchive) Status() providers.Status { return providers.Status{Kind: "wecom_archive", State: providers.StateLimited, Code: "archive.getchatdata_unimplemented"} }
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
  assert.equal(result.ok, true, result.errors.join('\n'));
  assert.ok(result.providers.some(({ file }) => file.endsWith('/linux_only.go')), JSON.stringify(result.providers));
});

test('scans linux go1.26 active implementations for the production image build set', async () => {
  const root = await writeFixture({
    'internal/modules/providers/archive/wecom/linux_go126.go': `//go:build linux && go1.26
package wecom
import "jiyi/mochat-go/internal/modules/providers"
type LinuxGo126Archive struct{}
func (LinuxGo126Archive) Kind() providers.Source { return providers.SourceExternal }
func (LinuxGo126Archive) Status() providers.Status { return providers.Status{Kind: "wecom_archive", State: providers.StateLimited, Code: "archive.getchatdata_unimplemented"} }
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
  assert.equal(result.ok, true, result.errors.join('\n'));
  assert.ok(result.providers.some(({ file }) => file.endsWith('/linux_go126.go')), JSON.stringify(result.providers));
});

test('rejects a simulation-only implementation for an external registration', async () => {
  const root = await writeFixture({
    'internal/modules/providers/archive/wecom/simulation_only.go': `//go:build linux && go1.26
package wecom
import "jiyi/mochat-go/internal/modules/providers"
type SimulationOnlyArchive struct{}
func (SimulationOnlyArchive) Kind() providers.Source { return providers.SourceSimulated }
func (SimulationOnlyArchive) Status() providers.Status { return providers.Status{Kind: "wecom_archive", Source: providers.SourceSimulated, State: providers.StateLimited, Code: "archive.simulation_ready"} }
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
  assert.match(result.errors.join('\n'), /SourceExternal|external active implementation/);
});

test('rejects raw or unrelated literals instead of treating them as active Status evidence', async () => {
  const root = await writeFixture({
    'internal/modules/providers/archive/wecom/unrelated_literal.go': `//go:build linux && go1.26
package wecom
import "jiyi/mochat-go/internal/modules/providers"
type UnrelatedLiteral struct{}
func (UnrelatedLiteral) Status() providers.Status { return providers.Status{Kind: "unrelated", State: providers.StateLimited} }
var rawStatusLiteral = providers.Status{Kind: "wecom_archive", State: providers.StateReady}
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
  assert.match(result.errors.join('\n'), /Status evidence|wecom_archive|external archive Status/);
});
