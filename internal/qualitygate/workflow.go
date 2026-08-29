// Package qualitygate validates the repository's backend CI contract.
package qualitygate

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/syntax"
)

const (
	workflowJobID = "mysql57-amd64"
	phase3Plan    = "docs/superpowers/plans/2026-07-29-phase3-yuanhu-business-foundation.md"
)

var requiredTriggerPaths = []string{
	".github/workflows/mysql57-amd64.yml",
	"architecture-policy.json",
	"Dockerfile",
	"cmd/mochat-architecture/**",
	"cmd/mochat-ai-insight-0165/**",
	"cmd/mochat-go/**",
	"go.mod",
	phase3Plan,
	"internal/**",
	"package.json",
	"pnpm-lock.yaml",
	"pnpm-workspace.yaml",
	"scripts/check_supply_chain_policy.mjs",
	"scripts/check_supply_chain_policy.test.mjs",
	"scripts/dev_check.sh",
	"scripts/test.sh",
	"scripts/audit_architecture_boundaries.sh",
	"scripts/architecture-size-baseline.txt",
	"scripts/test_audit_architecture_boundaries.sh",
	"scripts/test_backend_quality_gate_contract.sh",
	"scripts/ci_mysql57_amd64.sh",
	"scripts/lib/migration_inventory_smoke.sh",
	"scripts/preflight_0165_ai_daily_insight_unification.go",
	"scripts/smoke_schema_migrate.sh",
	"scripts/smoke_mysql57_schema_migrate.sh",
}

var environmentAssignmentPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)
var githubRunnerStatePattern = regexp.MustCompile(
	`(?i)(?:\bGITHUB_(?:ENV|PATH)\b|github\s*(?:\.\s*(?:env|path)\b|\[\s*['"](?:env|path)['"]\s*\]))`,
)

type workflowDocument struct {
	On          workflowTriggers       `yaml:"on"`
	Jobs        map[string]workflowJob `yaml:"jobs"`
	Env         map[string]string      `yaml:"env"`
	Defaults    workflowDefaults       `yaml:"defaults"`
	Permissions yaml.Node              `yaml:"permissions"`
}

type workflowTriggers struct {
	Push        workflowTrigger `yaml:"push"`
	PullRequest workflowTrigger `yaml:"pull_request"`
}

type workflowTrigger struct {
	Paths []string `yaml:"paths"`
}

type workflowJob struct {
	RunsOn          string            `yaml:"runs-on"`
	If              yaml.Node         `yaml:"if"`
	ContinueOnError yaml.Node         `yaml:"continue-on-error"`
	Needs           yaml.Node         `yaml:"needs"`
	Container       yaml.Node         `yaml:"container"`
	Defaults        workflowDefaults  `yaml:"defaults"`
	Steps           []workflowStep    `yaml:"steps"`
	Env             map[string]string `yaml:"env"`
	Permissions     yaml.Node         `yaml:"permissions"`
}

type workflowStep struct {
	Name             string            `yaml:"name"`
	Uses             string            `yaml:"uses"`
	Run              string            `yaml:"run"`
	If               yaml.Node         `yaml:"if"`
	ContinueOnError  yaml.Node         `yaml:"continue-on-error"`
	Shell            yaml.Node         `yaml:"shell"`
	WorkingDirectory yaml.Node         `yaml:"working-directory"`
	Env              map[string]string `yaml:"env"`
}

type workflowDefaults struct {
	Run workflowRunDefaults `yaml:"run"`
}

type workflowRunDefaults struct {
	Shell            yaml.Node `yaml:"shell"`
	WorkingDirectory yaml.Node `yaml:"working-directory"`
}

type requiredStep struct {
	name    string
	command string
}

var requiredSteps = []requiredStep{
	{name: "Supply-chain policy gate", command: "pnpm check:supply-chain"},
	{name: "Frontend dependency vulnerability gate", command: "pnpm audit --audit-level high"},
	{name: "Go architecture gate", command: "go run ./cmd/mochat-architecture -root ."},
	{name: "Go module race gate", command: "go test -race ./internal/modules/..."},
	{name: "Go tests", command: "go test ./..."},
	{name: "Go vet", command: "go vet ./..."},
	{name: "Go reachable vulnerability gate", command: "go run golang.org/x/vuln/cmd/govulncheck@v1.7.0 ./..."},
	{name: "Migration registry lifecycle gate", command: "bash ./scripts/smoke_schema_migrate.sh"},
	{name: "SCRM MySQL integration gate", command: "go test -v -count=1 -tags=integration ./internal/modules/scrm/adapters/mysql"},
}

var requiredStepEnvironments = map[string]map[string]string{
	"Migration registry lifecycle gate": {
		"MOCHAT_STACK_PROJECT": "mochat-go-schema-migrate-ci",
		"MOCHAT_MYSQL_PORT":    "13331",
	},
	"SCRM MySQL integration gate": {
		"MOCHAT_STACK_PROJECT":             "mochat-go-scrm-integration",
		"MOCHAT_MYSQL57_PORT":              "13333",
		"MOCHAT_MYSQL_DSN":                 "mochat:mochat_pass@tcp(127.0.0.1:13333)/mochat?parseTime=true&loc=UTC",
		"MOCHAT_REQUIRE_MYSQL_INTEGRATION": "1",
	},
}

var approvedNonRequiredRunSteps = map[string]string{
	"Install frontend dependencies":           "pnpm install --frozen-lockfile",
	"Frontend quick gate":                     "./scripts/frontend_check.sh quick",
	"Frontend build gate":                     "./scripts/frontend_check.sh build",
	"Architecture wrapper compatibility test": "sh ./scripts/test_audit_architecture_boundaries.sh",
	"Backend quality gate workflow contract":  "sh ./scripts/test_backend_quality_gate_contract.sh",
	"Checksum source dependency SBOM":         "sha256sum mochat-go.spdx.json | tee mochat-go.spdx.json.sha256",
	"Docker info":                             "docker info",
	"Architecture boundaries":                 "./scripts/audit_architecture_boundaries.sh\n./scripts/test_audit_architecture_boundaries.sh\n",
	"Run MySQL 5.7 amd64 gate":                "mkdir -p docs/phases/phase-pre0-standalone/evidence/ci\nenv -u GOROOT ./scripts/ci_mysql57_amd64.sh 2>&1 | tee docs/phases/phase-pre0-standalone/evidence/ci/mysql57-amd64.log\ngrep -q \"mysql57 amd64 CI gate passed\" docs/phases/phase-pre0-standalone/evidence/ci/mysql57-amd64.log\n",
	"Install Playwright Chromium":             "pnpm --filter @mochat/e2e exec playwright install --with-deps chromium",
	"Frontend browser gate":                   "./scripts/frontend_check.sh e2e",
}

// Validate checks the workflow and every repository file that owns part of the
// backend quality contract. Returned failures are deterministic.
func Validate(root string) []string {
	failures := validateWorkflow(filepath.Join(root, ".github", "workflows", "mysql57-amd64.yml"))

	files := map[string]string{}
	for _, path := range []string{
		"scripts/dev_check.sh",
		"scripts/test.sh",
		phase3Plan,
		"scripts/smoke_schema_migrate.sh",
		"scripts/lib/migration_inventory_smoke.sh",
	} {
		contents, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			failures = append(failures, fmt.Sprintf("read %s: %v", path, err))
			continue
		}
		files[path] = string(contents)
	}

	failures = append(failures, validateDeveloperScripts(files)...)
	failures = append(failures, validateLifecycle(
		files["scripts/smoke_schema_migrate.sh"],
		files["scripts/lib/migration_inventory_smoke.sh"],
	)...)
	failures = append(failures, validatePhase3Plan(files[phase3Plan])...)
	sort.Strings(failures)
	return failures
}

func validateWorkflow(path string) []string {
	contents, err := os.ReadFile(path)
	if err != nil {
		return []string{fmt.Sprintf("read workflow: %v", err)}
	}

	var document workflowDocument
	if err := yaml.Unmarshal(contents, &document); err != nil {
		return []string{fmt.Sprintf("parse workflow YAML: %v", err)}
	}

	failures := make([]string, 0)
	if !exactReadOnlyWorkflowPermissions(document.Permissions) {
		failures = append(failures, "workflow permissions must be exactly contents: read")
	}
	if !supportedWorkflowShell(document.Defaults.Run.Shell) {
		failures = append(
			failures,
			"workflow defaults.run.shell must provide supported bash errexit semantics",
		)
	}
	if !repositoryRootWorkingDirectory(document.Defaults.Run.WorkingDirectory) {
		failures = append(
			failures,
			"workflow defaults.run.working-directory must stay at repository root",
		)
	}
	if len(document.Env) != 0 {
		failures = append(failures, "workflow must not set environment overrides")
	}
	for name, paths := range map[string][]string{
		"push":         document.On.Push.Paths,
		"pull_request": document.On.PullRequest.Paths,
	} {
		actual := make(map[string]struct{}, len(paths))
		for _, path := range paths {
			actual[path] = struct{}{}
		}
		for _, required := range requiredTriggerPaths {
			if _, exists := actual[required]; !exists {
				failures = append(failures, fmt.Sprintf("on.%s.paths missing: %s", name, required))
			}
		}
	}

	job, exists := document.Jobs[workflowJobID]
	if !exists {
		failures = append(failures, "workflow missing exact job: "+workflowJobID)
		return failures
	}

	if job.RunsOn != "ubuntu-22.04" {
		failures = append(
			failures,
			"workflow job "+workflowJobID+" must use supported runner ubuntu-22.04",
		)
	}
	if job.Permissions.Kind != 0 {
		failures = append(failures, "workflow job "+workflowJobID+" must not override permissions")
	}
	if !yamlBooleanOrAbsent(job.If, true) {
		failures = append(failures, "workflow job "+workflowJobID+" must be unconditional")
	}
	if !yamlBooleanOrAbsent(job.ContinueOnError, false) {
		failures = append(failures, "workflow job "+workflowJobID+" must not continue on error")
	}
	if job.Needs.Kind != 0 {
		failures = append(
			failures,
			"workflow job "+workflowJobID+" must not depend on prerequisite jobs",
		)
	}
	if job.Container.Kind != 0 {
		failures = append(failures, "workflow job "+workflowJobID+" must not use a container")
	}
	if len(job.Env) != 0 {
		failures = append(
			failures,
			"workflow job "+workflowJobID+" must not set environment overrides",
		)
	}
	if !supportedWorkflowShell(job.Defaults.Run.Shell) {
		failures = append(
			failures,
			"workflow job "+workflowJobID+" defaults.run.shell must provide supported bash errexit semantics",
		)
	}
	if !repositoryRootWorkingDirectory(job.Defaults.Run.WorkingDirectory) {
		failures = append(
			failures,
			"workflow job "+workflowJobID+" defaults.run.working-directory must stay at repository root",
		)
	}
	for position, step := range job.Steps {
		if step.Run == "" {
			continue
		}
		name := step.Name
		if name == "" {
			name = fmt.Sprintf("#%d", position+1)
		}
		required := requiredWorkflowStep(step.Name)
		if !required && !approvedRunFingerprint(step.Name, step.Run) {
			failures = append(
				failures,
				"workflow job "+workflowJobID+" step "+name+
					" must match its approved run fingerprint",
			)
		}
		if !supportedWorkflowShell(step.Shell) {
			if required {
				failures = append(
					failures,
					step.Name+" step shell must provide supported bash errexit semantics",
				)
			} else {
				failures = append(
					failures,
					"workflow job "+workflowJobID+" step "+name+
						" shell must provide supported bash errexit semantics",
				)
			}
		}
		if !required && len(step.Env) != 0 {
			failures = append(
				failures,
				"workflow job "+workflowJobID+" step "+name+
					" must not set environment overrides",
			)
		}
		if writesGitHubRunnerEnvironmentState(step.Run) {
			failures = append(
				failures,
				"workflow job "+workflowJobID+" step "+name+
					" must not write GitHub runner environment state",
			)
		}
	}

	stepByName := make(map[string]workflowStep, len(job.Steps))
	positionByName := make(map[string]int, len(job.Steps))
	for position, step := range job.Steps {
		if step.Name == "" {
			continue
		}
		if _, duplicate := stepByName[step.Name]; duplicate {
			failures = append(
				failures,
				fmt.Sprintf("workflow job %s has duplicate step name %q", workflowJobID, step.Name),
			)
			continue
		}
		stepByName[step.Name] = step
		positionByName[step.Name] = position
	}

	positions := make([]int, 0, len(requiredSteps))
	for _, required := range requiredSteps {
		step, found := stepByName[required.name]
		if !found {
			failures = append(
				failures,
				fmt.Sprintf("workflow job %s missing named step: %s", workflowJobID, required.name),
			)
			continue
		}
		positions = append(positions, positionByName[required.name])
		if !containsExecutableCommandsInOrder(step.Run, []string{required.command}) {
			failures = append(
				failures,
				fmt.Sprintf("%s step does not own command: %s", required.name, required.command),
			)
		}
		if !yamlBooleanOrAbsent(step.If, true) {
			failures = append(failures, required.name+" step must be unconditional")
		}
		if !yamlBooleanOrAbsent(step.ContinueOnError, false) {
			failures = append(failures, required.name+" step must not continue on error")
		}
		if !repositoryRootWorkingDirectory(step.WorkingDirectory) {
			failures = append(
				failures,
				required.name+" step working-directory must stay at repository root",
			)
		}
		if !environmentMatches(step.Env, requiredStepEnvironments[required.name]) {
			failures = append(
				failures,
				required.name+" step environment must exactly match the required allowlist",
			)
		}
	}
	if len(positions) == len(requiredSteps) && !strictlyIncreasing(positions) {
		failures = append(
			failures,
			"workflow gate order must be supply policy -> frontend vulnerability -> architecture -> race -> full test -> vet -> Go vulnerability -> lifecycle -> integration within job "+workflowJobID,
		)
	}

	lifecycle := stepByName["Migration registry lifecycle gate"]
	if lifecycle.Env["MOCHAT_STACK_PROJECT"] != "mochat-go-schema-migrate-ci" {
		failures = append(failures, "migration lifecycle must use the dedicated mochat-go-schema-migrate-ci project")
	}
	if lifecycle.Env["MOCHAT_MYSQL_PORT"] != "13331" {
		failures = append(failures, "migration lifecycle must use dedicated port 13331")
	}
	if _, exists := lifecycle.Env["KEEP_STACK"]; exists {
		failures = append(failures, "migration lifecycle must retain default cleanup")
	}

	integration := stepByName["SCRM MySQL integration gate"]
	if integration.Env["MOCHAT_REQUIRE_MYSQL_INTEGRATION"] != "1" {
		failures = append(failures, "integration step must set strict require mode")
	}
	if integration.Env["MOCHAT_STACK_PROJECT"] != "mochat-go-scrm-integration" {
		failures = append(failures, "integration step must use its dedicated compose project")
	}
	if integration.Env["MOCHAT_MYSQL57_PORT"] != "13333" {
		failures = append(failures, "integration step must use dedicated port 13333")
	}
	integrationCleanup := `docker compose -p "$MOCHAT_STACK_PROJECT" -f deploy/mysql57/docker-compose.yml down -v --remove-orphans`
	integrationMarkers := []string{
		"trap cleanup EXIT",
		`docker compose -p "$MOCHAT_STACK_PROJECT" -f deploy/mysql57/docker-compose.yml up -d mysql57`,
	}
	if !containsExecutableCommandsInOrder(integration.Run, integrationMarkers) ||
		!shellFunctionExecutes(integration.Run, "cleanup", integrationCleanup) {
		failures = append(failures, "integration cleanup/down/trap must be defined before database startup")
	}

	if _, exists := document.Env["KEEP_STACK"]; exists {
		failures = append(failures, "workflow must not disable migration cleanup")
	}
	for jobName, workflowJob := range document.Jobs {
		if _, exists := workflowJob.Env["KEEP_STACK"]; exists {
			failures = append(failures, "workflow job "+jobName+" must not disable migration cleanup")
		}
		for _, step := range workflowJob.Steps {
			if _, exists := step.Env["KEEP_STACK"]; exists {
				failures = append(failures, "workflow step "+step.Name+" must not disable migration cleanup")
			}
		}
	}
	return failures
}

func validateDeveloperScripts(files map[string]string) []string {
	failures := make([]string, 0)
	build := "go build ./cmd/mochat-go ./cmd/mochat-inventory ./cmd/mochat-migrate ./cmd/mochat-bootstrap ./cmd/mochat-saas-maintenance ./cmd/mochat-architecture ./cmd/mochat-callback-legacy-cutover"
	devQuickScripts := literalCommandArguments(files["scripts/dev_check.sh"], "run_go")
	for _, required := range []string{
		"sh scripts/test_backend_quality_gate_contract.sh",
		build,
	} {
		if !anyScriptExecutes(devQuickScripts, required) {
			failures = append(failures, "scripts/dev_check.sh missing executable command: "+required)
		}
	}
	for _, required := range []string{
		"./scripts/test_backend_quality_gate_contract.sh",
		build,
	} {
		if !containsExecutableCommandsInOrder(files["scripts/test.sh"], []string{required}) {
			failures = append(failures, "scripts/test.sh missing executable command: "+required)
		}
	}
	return failures
}

func validateLifecycle(contents, inventoryLifecycle string) []string {
	failures := make([]string, 0)
	if !containsExecutableCommandsInOrder(contents, []string{
		"source scripts/lib/migration_inventory_smoke.sh",
		"run_migration_inventory_smoke",
	}) {
		failures = append(failures, "authoritative lifecycle wrapper must execute the shared runtime inventory smoke")
	}

	if strings.Contains(inventoryLifecycle, "0098_scrm_lead_foundation") ||
		!shellFunctionExecutes(inventoryLifecycle, "load_migration_inventory", `"$MIGRATE_BIN" -project-root "$PWD" -action inventory >"$INVENTORY_FILE"`) {
		failures = append(failures, "shared lifecycle must derive the migration registry from runtime inventory")
	}
	for _, stage := range []string{
		"build_migration_smoke_binaries",
		"load_migration_inventory",
		"apply_full_inventory",
		"verify_full_inventory_ledger",
		"verify_latest_rollback_reapply",
		"verify_checksum_drift_rejected",
		"verify_baseline_from_full_schema",
	} {
		if !shellFunctionExecutes(inventoryLifecycle, "run_migration_inventory_smoke", stage) {
			failures = append(failures, "shared lifecycle must execute inventory stage: "+stage)
		}
	}
	if !shellFunctionExecutes(inventoryLifecycle, "run_migration_inventory_smoke", `echo "migration inventory smoke passed: count=$INVENTORY_COUNT first=$INVENTORY_FIRST latest=$INVENTORY_LATEST checksum=$INVENTORY_LATEST_CHECKSUM kind=$INVENTORY_LATEST_KIND schema=$MYSQL_SCHEMA"`) {
		failures = append(failures, "shared lifecycle must emit dynamic count/first/latest/checksum/kind/schema evidence")
	}

	cleanupCommand := "compose down --remove-orphans >/dev/null 2>&1"
	cleanupMarkers := []string{
		"trap cleanup EXIT INT TERM",
		"start_mysql",
		"wait_healthy",
	}
	if !containsExecutableCommandsInOrder(contents, cleanupMarkers) ||
		!shellFunctionExecutes(contents, "start_mysql", "compose up -d mysql") ||
		!shellFunctionExecutes(contents, "cleanup", cleanupCommand) {
		failures = append(failures, "authoritative lifecycle cleanup trap must be installed before startup without deleting volumes")
	}
	return failures
}

func validatePhase3Plan(contents string) []string {
	failures := make([]string, 0)
	headings := regexp.MustCompile(`(?m)^### .+$`).FindAllStringIndex(contents, -1)
	if len(headings) < 7 {
		return []string{"Phase 3 plan must retain all seven task sections"}
	}
	task2 := contents[headings[1][0]:headings[2][0]]
	task7 := contents[headings[6][0]:]

	for _, literal := range []string{
		"`internal/modules/scrm/adapters/mysql/customer_lifecycle_repository_integration_test.go`",
		"`//go:build integration`",
		"TestSCRMCustomerLifecycleTenantIsolationIntegration",
		"TestSCRMCustomerLifecycleOptimisticLockIntegration",
		"TestSCRMCustomerLifecyclePublicPoolConcurrentClaimIntegration",
		"MOCHAT_REQUIRE_MYSQL_INTEGRATION=1 go test -v -count=1 -tags=integration ./internal/modules/scrm/adapters/mysql -run 'TestSCRMCustomerLifecycle(TenantIsolation|OptimisticLock|PublicPoolConcurrentClaim)Integration'",
	} {
		if !strings.Contains(task2, literal) {
			failures = append(failures, "Phase 3 Task 2 missing integration contract: "+literal)
		}
	}
	for _, literal := range []string{
		"go test ./...",
		"go vet ./...",
		"go run ./cmd/mochat-architecture -root .",
		"go test -race ./internal/modules/...",
		"MOCHAT_REQUIRE_MYSQL_INTEGRATION=1 go test -v -count=1 -tags=integration ./internal/modules/scrm/adapters/mysql",
		"bash ./scripts/smoke_schema_migrate.sh",
		"go build ./cmd/mochat-go ./cmd/mochat-inventory ./cmd/mochat-migrate ./cmd/mochat-bootstrap ./cmd/mochat-saas-maintenance ./cmd/mochat-architecture",
	} {
		if !strings.Contains(task7, literal) {
			failures = append(failures, "Phase 3 Task 7 missing final gate: "+literal)
		}
	}
	return failures
}

func strictlyIncreasing(values []int) bool {
	for index := 1; index < len(values); index++ {
		if values[index-1] >= values[index] {
			return false
		}
	}
	return true
}

func exactReadOnlyWorkflowPermissions(node yaml.Node) bool {
	if node.Kind != yaml.MappingNode || len(node.Content) != 2 {
		return false
	}
	key, value := node.Content[0], node.Content[1]
	return key.Kind == yaml.ScalarNode && key.Value == "contents" &&
		value.Kind == yaml.ScalarNode && value.Value == "read"
}

func yamlBooleanOrAbsent(node yaml.Node, expected bool) bool {
	if node.Kind == 0 {
		return true
	}
	return node.Kind == yaml.ScalarNode &&
		node.Tag == "!!bool" &&
		node.Value == strconv.FormatBool(expected)
}

func supportedWorkflowShell(node yaml.Node) bool {
	if node.Kind == 0 {
		return true
	}
	if node.Kind != yaml.ScalarNode || node.Tag != "!!str" {
		return false
	}
	switch node.Value {
	case "bash",
		"bash -e {0}",
		"bash -eo pipefail {0}",
		"bash -e -o pipefail {0}",
		"bash --noprofile --norc -eo pipefail {0}",
		"bash --noprofile --norc -e -o pipefail {0}":
		return true
	default:
		return false
	}
}

func repositoryRootWorkingDirectory(node yaml.Node) bool {
	if node.Kind == 0 {
		return true
	}
	return node.Kind == yaml.ScalarNode &&
		node.Tag == "!!str" &&
		node.Value == "."
}

func environmentMatches(actual, expected map[string]string) bool {
	if len(actual) != len(expected) {
		return false
	}
	for name, expectedValue := range expected {
		if actual[name] != expectedValue {
			return false
		}
	}
	return true
}

func approvedRunFingerprint(name, contents string) bool {
	approved, exists := approvedNonRequiredRunSteps[name]
	if !exists {
		return false
	}
	return sha256.Sum256([]byte(contents)) == sha256.Sum256([]byte(approved))
}

func requiredWorkflowStep(name string) bool {
	for _, required := range requiredSteps {
		if required.name == name {
			return true
		}
	}
	return false
}

func writesGitHubRunnerEnvironmentState(contents string) bool {
	return writesGitHubRunnerEnvironmentStateDepth(contents, 0)
}

func writesGitHubRunnerEnvironmentStateDepth(contents string, depth int) bool {
	file, err := parseShell(contents)
	if err != nil {
		return githubRunnerStatePattern.MatchString(contents)
	}

	aliases := runnerStateAliases(file)
	writesState := false
	syntax.Walk(file, func(node syntax.Node) bool {
		if writesState || node == nil {
			return !writesState
		}
		switch typed := node.(type) {
		case *syntax.Redirect:
			if outputRedirect(typed.Op) &&
				shellWordReferencesRunnerState(typed.Word, aliases) {
				writesState = true
				return false
			}
		case *syntax.CallExpr:
			if shellCallMayWriteRunnerState(typed, aliases, depth) {
				writesState = true
				return false
			}
		}
		return true
	})
	return writesState
}

func runnerStateAliases(file *syntax.File) map[string]struct{} {
	aliases := make(map[string]struct{})
	knownValues := make(map[string]string)
	for pass := 0; pass < 64; pass++ {
		changed := false
		syntax.Walk(file, func(node syntax.Node) bool {
			assignment, ok := node.(*syntax.Assign)
			if !ok || assignment.Name == nil {
				return true
			}
			name := assignment.Name.Value
			if assignment.Value != nil {
				if value, known := staticShellWordValue(assignment.Value, knownValues); known {
					if assignment.Append {
						base, exists := knownValues[name]
						if !exists {
							return true
						}
						value = base + value
					}
					if knownValues[name] != value {
						knownValues[name] = value
						changed = true
					}
					if githubRunnerStatePattern.MatchString(value) {
						if _, exists := aliases[name]; !exists {
							aliases[name] = struct{}{}
							changed = true
						}
					}
				}
			}
			valueReferencesState := assignment.Value != nil &&
				shellWordReferencesRunnerState(assignment.Value, aliases)
			arrayReferencesState := assignment.Array != nil &&
				shellNodeReferencesRunnerState(assignment.Array, aliases)
			if valueReferencesState || arrayReferencesState {
				if _, exists := aliases[name]; !exists {
					aliases[name] = struct{}{}
					changed = true
				}
			}
			return true
		})
		if !changed {
			return aliases
		}
	}
	return aliases
}

func staticShellWordValue(word *syntax.Word, knownValues map[string]string) (string, bool) {
	if word == nil {
		return "", false
	}
	return staticShellWordPartsValue(word.Parts, knownValues)
}

func staticShellWordPartsValue(
	parts []syntax.WordPart,
	knownValues map[string]string,
) (string, bool) {
	var value strings.Builder
	for _, part := range parts {
		switch typed := part.(type) {
		case *syntax.Lit:
			value.WriteString(typed.Value)
		case *syntax.SglQuoted:
			value.WriteString(typed.Value)
		case *syntax.DblQuoted:
			nested, known := staticShellWordPartsValue(typed.Parts, knownValues)
			if !known {
				return "", false
			}
			value.WriteString(nested)
		case *syntax.ParamExp:
			if !simpleShellParameterExpansion(typed) {
				return "", false
			}
			known, exists := knownValues[typed.Param.Value]
			if !exists {
				return "", false
			}
			value.WriteString(known)
		default:
			return "", false
		}
	}
	return value.String(), true
}

func simpleShellParameterExpansion(parameter *syntax.ParamExp) bool {
	return parameter != nil &&
		parameter.Param != nil &&
		parameter.Flags == nil &&
		!parameter.Excl &&
		!parameter.Length &&
		!parameter.Width &&
		!parameter.IsSet &&
		parameter.NestedParam == nil &&
		parameter.Index == nil &&
		len(parameter.Modifiers) == 0 &&
		parameter.Slice == nil &&
		parameter.Repl == nil &&
		parameter.Names == 0 &&
		parameter.Exp == nil
}

func shellWordReferencesRunnerState(word *syntax.Word, aliases map[string]struct{}) bool {
	if word == nil {
		return false
	}
	return shellNodeReferencesRunnerState(word, aliases)
}

func shellNodeReferencesRunnerState(node syntax.Node, aliases map[string]struct{}) bool {
	if node == nil {
		return false
	}
	referencesState := false
	syntax.Walk(node, func(nested syntax.Node) bool {
		if referencesState || nested == nil {
			return !referencesState
		}
		switch typed := nested.(type) {
		case *syntax.ParamExp:
			if typed.Param != nil {
				if githubRunnerStatePattern.MatchString(typed.Param.Value) {
					referencesState = true
					return false
				}
				if _, exists := aliases[typed.Param.Value]; exists {
					referencesState = true
					return false
				}
			}
		case *syntax.Lit:
			referencesState = githubRunnerStatePattern.MatchString(typed.Value)
		}
		return !referencesState
	})
	return referencesState
}

func outputRedirect(operator syntax.RedirOperator) bool {
	switch operator {
	case syntax.RdrOut,
		syntax.AppOut,
		syntax.RdrInOut,
		syntax.DplOut,
		syntax.RdrClob,
		syntax.AppClob,
		syntax.RdrAll,
		syntax.RdrAllClob,
		syntax.AppAll,
		syntax.AppAllClob:
		return true
	default:
		return false
	}
}

func shellCallMayWriteRunnerState(
	call *syntax.CallExpr,
	aliases map[string]struct{},
	depth int,
) bool {
	if len(call.Args) == 0 {
		return false
	}
	if nestedShellMayWriteRunnerState(call, depth) {
		return true
	}
	referencesState := false
	for _, argument := range call.Args {
		if shellWordReferencesRunnerState(argument, aliases) {
			referencesState = true
			break
		}
	}
	if !referencesState {
		for _, assignment := range call.Assigns {
			if assignment.Value != nil &&
				shellWordReferencesRunnerState(assignment.Value, aliases) {
				referencesState = true
				break
			}
		}
	}
	if !referencesState {
		return false
	}

	switch shellCallCommandName(call.Args) {
	case "printf":
		for _, argument := range call.Args {
			if argument.Lit() == "-v" {
				return true
			}
		}
		return false
	case "echo", "printenv", "cat", "grep", "test", "stat", "ls",
		"readlink", "realpath", "dirname", "basename", "wc", "head", "tail",
		"cmp", "diff", "sha256sum":
		return false
	default:
		return true
	}
}

func shellCallCommandName(arguments []*syntax.Word) string {
	name, _ := shellCallCommand(arguments)
	return name
}

func shellCallCommand(arguments []*syntax.Word) (string, int) {
	index := 0
	for index < len(arguments) {
		switch arguments[index].Lit() {
		case "builtin":
			index++
		case "command":
			index++
			for index < len(arguments) {
				option := arguments[index].Lit()
				if option == "--" {
					index++
					break
				}
				if option != "-p" && option != "-v" && option != "-V" {
					break
				}
				index++
			}
		case "env":
			index++
			for index < len(arguments) {
				option := arguments[index].Lit()
				switch {
				case option == "--":
					index++
					break
				case option == "-u" || option == "--unset":
					index += 2
					continue
				case strings.HasPrefix(option, "--unset="),
					option == "-i",
					option == "--ignore-environment",
					environmentAssignmentPattern.MatchString(option):
					index++
					continue
				}
				break
			}
		default:
			return arguments[index].Lit(), index
		}
	}
	return "", -1
}

func nestedShellMayWriteRunnerState(call *syntax.CallExpr, depth int) bool {
	name, commandIndex := shellCallCommand(call.Args)
	if commandIndex < 0 {
		return false
	}
	switch name {
	case "bash", "sh", "dash", "ksh", "zsh":
		payloadIndex := shellCommandPayloadIndex(call.Args, commandIndex+1, "-c")
		if payloadIndex < 0 {
			return false
		}
		if depth >= 8 || payloadIndex >= len(call.Args) {
			return true
		}
		payload, literal := literalShellWord(call.Args[payloadIndex])
		return !literal || writesGitHubRunnerEnvironmentStateDepth(payload, depth+1)
	case "eval":
		if depth >= 8 || commandIndex+1 >= len(call.Args) {
			return true
		}
		payload := make([]string, 0, len(call.Args)-commandIndex-1)
		for _, argument := range call.Args[commandIndex+1:] {
			literal, known := literalShellWord(argument)
			if !known {
				return true
			}
			payload = append(payload, literal)
		}
		return writesGitHubRunnerEnvironmentStateDepth(strings.Join(payload, " "), depth+1)
	case "python", "python3":
		return shellCommandPayloadIndex(call.Args, commandIndex+1, "-c") >= 0
	case "node", "ruby", "perl":
		return shellCommandPayloadIndex(call.Args, commandIndex+1, "-e") >= 0
	case "pwsh", "powershell":
		for _, argument := range call.Args[commandIndex+1:] {
			switch strings.ToLower(argument.Lit()) {
			case "-command", "-encodedcommand", "-c":
				return true
			}
		}
	}
	return false
}

func shellCommandPayloadIndex(
	arguments []*syntax.Word,
	start int,
	shortOption string,
) int {
	for index := start; index < len(arguments); index++ {
		option := arguments[index].Lit()
		if option == shortOption {
			return index + 1
		}
		if strings.HasPrefix(option, "-") &&
			!strings.HasPrefix(option, "--") &&
			strings.Contains(option[1:], strings.TrimPrefix(shortOption, "-")) {
			return index + 1
		}
	}
	return -1
}

type shellStatement struct {
	source         *syntax.Stmt
	candidates     []string
	errexitEnabled bool
	inFinalRoot    bool
}

func containsExecutableCommandsInOrder(contents string, expected []string) bool {
	file, err := parseShell(contents)
	if err != nil {
		return false
	}
	statements := safeTopLevelShellStatements(file)
	position := 0
	for _, command := range expected {
		canonical, ok := canonicalShellCommand(command)
		if !ok {
			return false
		}
		found := false
		for position < len(statements) {
			statement := statements[position]
			position++
			for _, candidate := range statement.candidates {
				if candidate == canonical &&
					(statement.errexitEnabled || statement.inFinalRoot) {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func shellFunctionExecutes(contents, functionName, expected string) bool {
	file, err := parseShell(contents)
	if err != nil {
		return false
	}
	canonical, ok := canonicalShellCommand(expected)
	if !ok {
		return false
	}
	var matched *syntax.FuncDecl
	for _, statement := range file.Stmts {
		function, ok := statement.Cmd.(*syntax.FuncDecl)
		if !ok || function.Name == nil || function.Name.Value != functionName {
			continue
		}
		if matched != nil ||
			statement.Semicolon.IsValid() ||
			statement.Negated ||
			statement.Background ||
			statement.Coprocess ||
			statement.Disown {
			return false
		}
		matched = function
	}
	return matched != nil && functionStatementExecutes(matched.Body, canonical)
}

func functionStatementExecutes(statement *syntax.Stmt, canonical string) bool {
	if statement == nil ||
		statement.Semicolon.IsValid() ||
		statement.Negated ||
		statement.Background ||
		statement.Coprocess ||
		statement.Disown {
		return false
	}

	switch command := statement.Cmd.(type) {
	case *syntax.CallExpr:
		shell, ok := shellStatementFromCall(statement, command)
		if !ok {
			return false
		}
		return stringSliceContains(shell.candidates, canonical)
	case *syntax.Block:
		return functionStatementsExecute(command.Stmts, canonical)
	case *syntax.IfClause:
		return functionIfClauseExecutes(command, canonical)
	case *syntax.BinaryCmd:
		if functionStatementExecutes(command.X, canonical) {
			return true
		}
		truth, known := staticShellStatementTruth(command.X)
		switch command.Op {
		case syntax.AndStmt:
			return known && truth && functionStatementExecutes(command.Y, canonical)
		case syntax.OrStmt:
			return known && !truth && functionStatementExecutes(command.Y, canonical)
		default:
			return false
		}
	default:
		return false
	}
}

func functionStatementsExecute(statements []*syntax.Stmt, canonical string) bool {
	for _, statement := range statements {
		if functionStatementExecutes(statement, canonical) {
			return true
		}
		if shellStatementTerminates(statement) {
			return false
		}
	}
	return false
}

func functionIfClauseExecutes(clause *syntax.IfClause, canonical string) bool {
	if clause == nil {
		return false
	}
	if len(clause.Cond) == 0 {
		return functionStatementsExecute(clause.Then, canonical)
	}
	if truth, known := staticShellStatementsTruth(clause.Cond); known {
		if truth {
			return functionStatementsExecute(clause.Then, canonical)
		}
		return functionIfClauseExecutes(clause.Else, canonical)
	}
	return functionStatementsExecute(clause.Then, canonical) ||
		functionIfClauseExecutes(clause.Else, canonical)
}

func staticShellStatementsTruth(statements []*syntax.Stmt) (bool, bool) {
	if len(statements) != 1 {
		return false, false
	}
	return staticShellStatementTruth(statements[0])
}

func staticShellStatementTruth(statement *syntax.Stmt) (bool, bool) {
	if statement == nil ||
		statement.Background ||
		statement.Coprocess ||
		statement.Disown {
		return false, false
	}

	var truth bool
	switch command := statement.Cmd.(type) {
	case *syntax.CallExpr:
		if len(command.Args) != 1 || len(command.Assigns) != 0 {
			return false, false
		}
		switch command.Args[0].Lit() {
		case "true", ":":
			truth = true
		case "false":
			truth = false
		default:
			return false, false
		}
	case *syntax.BinaryCmd:
		left, leftKnown := staticShellStatementTruth(command.X)
		if !leftKnown {
			return false, false
		}
		switch command.Op {
		case syntax.AndStmt:
			if !left {
				truth = false
				break
			}
			right, rightKnown := staticShellStatementTruth(command.Y)
			if !rightKnown {
				return false, false
			}
			truth = right
		case syntax.OrStmt:
			if left {
				truth = true
				break
			}
			right, rightKnown := staticShellStatementTruth(command.Y)
			if !rightKnown {
				return false, false
			}
			truth = right
		default:
			return false, false
		}
	default:
		return false, false
	}
	if statement.Negated {
		truth = !truth
	}
	return truth, true
}

func shellStatementTerminates(statement *syntax.Stmt) bool {
	if statement == nil ||
		statement.Negated ||
		statement.Background ||
		statement.Coprocess ||
		statement.Disown {
		return false
	}
	call, ok := statement.Cmd.(*syntax.CallExpr)
	if !ok || len(call.Args) == 0 {
		return false
	}
	commandIndex := 0
	if call.Args[0].Lit() == "builtin" || call.Args[0].Lit() == "command" {
		if len(call.Args) < 2 {
			return false
		}
		commandIndex = 1
	}
	switch call.Args[commandIndex].Lit() {
	case "exit", "return":
		return true
	case "exec":
		return execCommandIndex(call.Args[commandIndex:]) >= 0
	default:
		return false
	}
}

func literalCommandArguments(contents, commandName string) []string {
	file, err := parseShell(contents)
	if err != nil {
		return nil
	}
	arguments := make([]string, 0)
	syntax.Walk(file, func(node syntax.Node) bool {
		if node == nil {
			return true
		}
		statement, ok := node.(*syntax.Stmt)
		if !ok {
			return true
		}
		call, ok := statement.Cmd.(*syntax.CallExpr)
		if !ok || len(call.Args) < 2 || call.Args[0].Lit() != commandName {
			return true
		}
		for _, argument := range call.Args[1:] {
			if literal, ok := literalShellWord(argument); ok {
				arguments = append(arguments, literal)
			}
		}
		return true
	})
	return arguments
}

func literalShellWord(word *syntax.Word) (string, bool) {
	literal, err := expand.Literal(nil, word)
	return literal, err == nil
}

func anyScriptExecutes(scripts []string, expected string) bool {
	for _, script := range scripts {
		if containsExecutableCommandsInOrder(script, []string{expected}) {
			return true
		}
	}
	return false
}

func parseShell(contents string) (*syntax.File, error) {
	parser := syntax.NewParser(syntax.Variant(syntax.LangBash))
	return parser.Parse(strings.NewReader(contents), "")
}

func safeTopLevelShellStatements(file *syntax.File) []shellStatement {
	statements := make([]shellStatement, 0, len(file.Stmts))
	errexitEnabled := true
	for rootIndex, statement := range file.Stmts {
		safe, ok := safeSerialShellStatements(statement)
		if !ok {
			if enabled, changed := shellStatementErrexitChange(statement); changed {
				errexitEnabled = enabled
			} else if shellStatementMayDisableErrexit(statement) {
				errexitEnabled = false
			}
			if shellStatementMayTerminateSuccessfully(statement) {
				return statements
			}
			continue
		}
		for _, shell := range safe {
			shell.errexitEnabled = errexitEnabled
			shell.inFinalRoot = rootIndex == len(file.Stmts)-1
			statements = append(statements, shell)
			if enabled, changed := shellStatementErrexitChange(shell.source); changed {
				errexitEnabled = enabled
			}
			if shellStatementTerminates(shell.source) {
				return statements
			}
		}
	}
	return statements
}

func shellStatementMayDisableErrexit(statement *syntax.Stmt) bool {
	disabled := false
	syntax.Walk(statement, func(node syntax.Node) bool {
		if disabled || node == nil {
			return !disabled
		}
		switch node.(type) {
		case *syntax.FuncDecl, *syntax.Subshell, *syntax.CmdSubst, *syntax.ProcSubst:
			return false
		}
		nested, ok := node.(*syntax.Stmt)
		if !ok {
			return true
		}
		enabled, changed := shellStatementErrexitChange(nested)
		if changed && !enabled {
			disabled = true
			return false
		}
		return true
	})
	return disabled
}

func shellStatementMayTerminateSuccessfully(statement *syntax.Stmt) bool {
	terminates := false
	syntax.Walk(statement, func(node syntax.Node) bool {
		if terminates || node == nil {
			return !terminates
		}
		switch node.(type) {
		case *syntax.FuncDecl, *syntax.Subshell, *syntax.CmdSubst, *syntax.ProcSubst:
			return false
		}
		nested, ok := node.(*syntax.Stmt)
		if !ok {
			return true
		}
		if shellStatementCanTerminateSuccessfully(nested) {
			terminates = true
			return false
		}
		return true
	})
	return terminates
}

func shellStatementCanTerminateSuccessfully(statement *syntax.Stmt) bool {
	if statement == nil ||
		statement.Background ||
		statement.Coprocess ||
		statement.Disown {
		return false
	}
	call, ok := statement.Cmd.(*syntax.CallExpr)
	if !ok || len(call.Args) == 0 {
		return false
	}
	commandIndex := 0
	if call.Args[0].Lit() == "builtin" || call.Args[0].Lit() == "command" {
		if len(call.Args) < 2 {
			return false
		}
		commandIndex = 1
	}
	switch call.Args[commandIndex].Lit() {
	case "exec":
		return execCommandIndex(call.Args[commandIndex:]) >= 0
	case "exit", "return":
		statusIndex := commandIndex + 1
		if statusIndex >= len(call.Args) {
			return true
		}
		status, err := strconv.Atoi(call.Args[statusIndex].Lit())
		return err != nil || status%256 == 0
	default:
		return false
	}
}

func safeSerialShellStatements(statement *syntax.Stmt) ([]shellStatement, bool) {
	if statement == nil ||
		statement.Semicolon.IsValid() ||
		statement.Negated ||
		statement.Background ||
		statement.Coprocess ||
		statement.Disown {
		return nil, false
	}

	switch command := statement.Cmd.(type) {
	case *syntax.CallExpr:
		shell, ok := shellStatementFromCall(statement, command)
		if !ok {
			return nil, false
		}
		return []shellStatement{shell}, true
	case *syntax.BinaryCmd:
		if command.Op != syntax.AndStmt {
			return nil, false
		}
		left, leftOK := safeSerialShellStatements(command.X)
		right, rightOK := safeSerialShellStatements(command.Y)
		if !leftOK || !rightOK {
			return nil, false
		}
		return append(left, right...), true
	default:
		return nil, false
	}
}

func shellStatementFromCall(statement *syntax.Stmt, call *syntax.CallExpr) (shellStatement, bool) {
	if len(call.Args) == 0 {
		return shellStatement{}, false
	}
	candidates := []string{printShellNode(statement)}

	if len(call.Assigns) != 0 {
		withoutAssignments := cloneStatementWithCall(statement, call)
		withoutAssignmentsCall := withoutAssignments.Cmd.(*syntax.CallExpr)
		withoutAssignmentsCall.Assigns = nil
		candidates = append(candidates, printShellNode(withoutAssignments))
	}
	if stripped := stripEnvCommand(statement, call); stripped != nil {
		candidates = append(candidates, printShellNode(stripped))
	}
	if stripped := stripExecCommand(statement, call); stripped != nil {
		candidates = append(candidates, printShellNode(stripped))
		strippedCall := stripped.Cmd.(*syntax.CallExpr)
		if strippedEnv := stripEnvCommand(stripped, strippedCall); strippedEnv != nil {
			candidates = append(candidates, printShellNode(strippedEnv))
		}
	}
	candidates = uniqueStrings(candidates)
	return shellStatement{
		source:     statement,
		candidates: candidates,
	}, len(candidates) != 0
}

func shellStatementErrexitChange(statement *syntax.Stmt) (bool, bool) {
	if statement == nil {
		return false, false
	}
	call, ok := statement.Cmd.(*syntax.CallExpr)
	if !ok || len(call.Args) == 0 {
		return false, false
	}
	index := 0
	if call.Args[0].Lit() == "builtin" || call.Args[0].Lit() == "command" {
		index++
	}
	if index >= len(call.Args) || call.Args[index].Lit() != "set" {
		return false, false
	}
	index++

	enabled := false
	changed := false
	for index < len(call.Args) {
		option := call.Args[index].Lit()
		switch {
		case option == "-o" || option == "+o":
			if index+1 < len(call.Args) && call.Args[index+1].Lit() == "errexit" {
				enabled = option == "-o"
				changed = true
				index += 2
				continue
			}
		case strings.HasPrefix(option, "-") && strings.Contains(option[1:], "e"):
			enabled = true
			changed = true
		case strings.HasPrefix(option, "+") && strings.Contains(option[1:], "e"):
			enabled = false
			changed = true
		}
		index++
	}
	return enabled, changed
}

func stripExecCommand(statement *syntax.Stmt, call *syntax.CallExpr) *syntax.Stmt {
	index := execCommandIndex(call.Args)
	if index < 0 {
		return nil
	}
	clone := cloneStatementWithCall(statement, call)
	cloneCall := clone.Cmd.(*syntax.CallExpr)
	cloneCall.Assigns = nil
	cloneCall.Args = append([]*syntax.Word(nil), call.Args[index:]...)
	return clone
}

func execCommandIndex(arguments []*syntax.Word) int {
	if len(arguments) < 2 || arguments[0].Lit() != "exec" {
		return -1
	}
	index := 1
	for index < len(arguments) {
		option := arguments[index].Lit()
		switch {
		case option == "--":
			index++
			if index < len(arguments) {
				return index
			}
			return -1
		case option == "-a":
			index += 2
		case option == "-c" || option == "-l":
			index++
		case strings.HasPrefix(option, "-"):
			index++
		default:
			return index
		}
	}
	return -1
}

func stripEnvCommand(statement *syntax.Stmt, call *syntax.CallExpr) *syntax.Stmt {
	if len(call.Args) < 2 || call.Args[0].Lit() != "env" {
		return nil
	}
	index := 1
	for index < len(call.Args) {
		literal := call.Args[index].Lit()
		switch {
		case literal == "--":
			index++
			goto command
		case literal == "-u" || literal == "--unset":
			index += 2
		case strings.HasPrefix(literal, "--unset="), literal == "-i", literal == "--ignore-environment":
			index++
		case environmentAssignmentPattern.MatchString(literal):
			index++
		default:
			goto command
		}
	}

command:
	if index >= len(call.Args) {
		return nil
	}
	clone := cloneStatementWithCall(statement, call)
	cloneCall := clone.Cmd.(*syntax.CallExpr)
	cloneCall.Assigns = nil
	cloneCall.Args = append([]*syntax.Word(nil), call.Args[index:]...)
	return clone
}

func cloneStatementWithCall(statement *syntax.Stmt, call *syntax.CallExpr) *syntax.Stmt {
	statementClone := *statement
	callClone := *call
	statementClone.Cmd = &callClone
	return &statementClone
}

func canonicalShellCommand(command string) (string, bool) {
	file, err := parseShell(command)
	if err != nil {
		return "", false
	}
	statements := safeTopLevelShellStatements(file)
	if len(statements) != 1 {
		return "", false
	}
	return statements[0].candidates[0], true
}

func printShellNode(node syntax.Node) string {
	var output bytes.Buffer
	printer := syntax.NewPrinter(syntax.Minify(true))
	if err := printer.Print(&output, node); err != nil {
		return ""
	}
	return strings.TrimSpace(output.String())
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func stringSliceContains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
