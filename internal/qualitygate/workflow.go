// Package qualitygate validates the repository's backend CI contract.
package qualitygate

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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
	"cmd/mochat-architecture/**",
	"cmd/mochat-go/**",
	phase3Plan,
	"internal/**",
	"scripts/dev_check.sh",
	"scripts/test.sh",
	"scripts/audit_architecture_boundaries.sh",
	"scripts/architecture-size-baseline.txt",
	"scripts/test_audit_architecture_boundaries.sh",
	"scripts/test_backend_quality_gate_contract.sh",
	"scripts/ci_mysql57_amd64.sh",
	"scripts/smoke_schema_migrate.sh",
	"scripts/smoke_mysql57_schema_migrate.sh",
}

var environmentAssignmentPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)

type workflowDocument struct {
	On   workflowTriggers       `yaml:"on"`
	Jobs map[string]workflowJob `yaml:"jobs"`
	Env  map[string]string      `yaml:"env"`
}

type workflowTriggers struct {
	Push        workflowTrigger `yaml:"push"`
	PullRequest workflowTrigger `yaml:"pull_request"`
}

type workflowTrigger struct {
	Paths []string `yaml:"paths"`
}

type workflowJob struct {
	Steps []workflowStep    `yaml:"steps"`
	Env   map[string]string `yaml:"env"`
}

type workflowStep struct {
	Name string            `yaml:"name"`
	Uses string            `yaml:"uses"`
	Run  string            `yaml:"run"`
	Env  map[string]string `yaml:"env"`
}

type requiredStep struct {
	name    string
	command string
}

var requiredSteps = []requiredStep{
	{name: "Go architecture gate", command: "go run ./cmd/mochat-architecture -root ."},
	{name: "Go module race gate", command: "go test -race ./internal/modules/..."},
	{name: "Go tests", command: "go test ./..."},
	{name: "Go vet", command: "go vet ./..."},
	{name: "Migration 0098 lifecycle gate", command: "bash ./scripts/smoke_schema_migrate.sh"},
	{name: "SCRM MySQL integration gate", command: "go test -v -count=1 -tags=integration ./internal/modules/scrm/adapters/mysql"},
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
	} {
		contents, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			failures = append(failures, fmt.Sprintf("read %s: %v", path, err))
			continue
		}
		files[path] = string(contents)
	}

	failures = append(failures, validateDeveloperScripts(files)...)
	failures = append(failures, validateLifecycle(files["scripts/smoke_schema_migrate.sh"])...)
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
	}
	if len(positions) == len(requiredSteps) && !strictlyIncreasing(positions) {
		failures = append(
			failures,
			"workflow gate order must be architecture -> race -> full test -> vet -> lifecycle -> integration within job "+workflowJobID,
		)
	}

	lifecycle := stepByName["Migration 0098 lifecycle gate"]
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
		integrationCleanup,
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
	build := "go build ./cmd/mochat-go ./cmd/mochat-inventory ./cmd/mochat-migrate ./cmd/mochat-bootstrap ./cmd/mochat-saas-maintenance ./cmd/mochat-architecture"
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

func validateLifecycle(contents string) []string {
	failures := make([]string, 0)
	markers := []string{
		`"$MIGRATE_BIN" -dsn "$MIGRATE_DSN" -project-root "$PWD" -action apply >"$WORK_DIR/apply.out"`,
		`test "$(mysql_scalar mochat_migrate_check "SELECT COUNT(*) FROM mochat_go_schema_migrations WHERE version = '0098_scrm_lead_foundation' AND CHAR_LENGTH(checksum) = 64")" = "1"`,
		`"$MIGRATE_BIN" -dsn "$MIGRATE_DSN" -project-root "$PWD" -action rollback >"$WORK_DIR/rollback-0098.out"`,
		`grep -q $'0098_scrm_lead_foundation\trolled_back' "$WORK_DIR/rollback-0098.out"`,
		`test "$(mysql_scalar mochat_migrate_check "SHOW TABLES LIKE 'mochat_go_scrm_leads'")" = ""`,
		`"$MIGRATE_BIN" -dsn "$MIGRATE_DSN" -project-root "$PWD" -action apply >"$WORK_DIR/reapply-latest.out"`,
		`grep -q $'0098_scrm_lead_foundation\tapplied_now' "$WORK_DIR/reapply-latest.out"`,
	}
	if !containsExecutableCommandsInOrder(contents, markers) {
		failures = append(failures, "authoritative lifecycle script must execute 0098 apply/checksum/rollback/replay in order")
	}

	cleanupCommand := "compose down -v --remove-orphans >/dev/null 2>&1"
	cleanupMarkers := []string{
		cleanupCommand,
		"trap cleanup EXIT INT TERM",
		"compose up -d mysql",
	}
	if !containsExecutableCommandsInOrder(contents, cleanupMarkers) ||
		!shellFunctionExecutes(contents, "cleanup", cleanupCommand) {
		failures = append(failures, "authoritative lifecycle cleanup trap must be installed before startup")
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

type shellStatement struct {
	position   uint
	candidates []string
}

func containsExecutableCommandsInOrder(contents string, expected []string) bool {
	file, err := parseShell(contents)
	if err != nil {
		return false
	}
	statements := shellStatements(file)
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
				if candidate == canonical {
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
	matched := false
	syntax.Walk(file, func(node syntax.Node) bool {
		if matched || node == nil {
			return !matched
		}
		function, ok := node.(*syntax.FuncDecl)
		if !ok || function.Name == nil || function.Name.Value != functionName {
			return true
		}
		for _, statement := range shellStatements(function.Body) {
			for _, candidate := range statement.candidates {
				if candidate == canonical {
					matched = true
					return false
				}
			}
		}
		return false
	})
	return matched
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

func shellStatements(node syntax.Node) []shellStatement {
	statements := make([]shellStatement, 0)
	syntax.Walk(node, func(node syntax.Node) bool {
		if node == nil {
			return true
		}
		statement, ok := node.(*syntax.Stmt)
		if !ok {
			return true
		}
		call, ok := statement.Cmd.(*syntax.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
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
		statements = append(statements, shellStatement{
			position:   statement.Pos().Offset(),
			candidates: uniqueStrings(candidates),
		})
		return true
	})
	sort.SliceStable(statements, func(left, right int) bool {
		return statements[left].position < statements[right].position
	})
	return statements
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
	statements := shellStatements(file)
	if len(statements) == 0 {
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
