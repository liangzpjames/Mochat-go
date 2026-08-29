package taskrunner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"jiyi/mochat-go/internal/observability"
)

const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusSucceeded = "succeeded"
	StatusStopped   = "stopped"
	StatusFailed    = "failed"

	ExecutionKindPeriodicTick = "periodic_tick"
	ExecutionKindQueueItem    = "queue_item"
)

type Task struct {
	Name                   string
	Run                    func(context.Context) error
	Periodic               bool
	PeriodicReadinessGrace time.Duration
}

type PeriodicConfig struct {
	Name                string
	Interval            time.Duration
	RunOnStart          bool
	Logger              *slog.Logger
	SuppressOutcomeLogs bool
}

type Snapshot struct {
	Name                string             `json:"name"`
	RunID               string             `json:"run_id,omitempty"`
	Status              string             `json:"status"`
	StartedAt           string             `json:"started_at"`
	StoppedAt           string             `json:"stopped_at,omitempty"`
	Error               string             `json:"error,omitempty"`
	LastSuccessAt       string             `json:"last_success_at,omitempty"`
	ConsecutiveFailures int                `json:"consecutive_failures"`
	LatestExecution     *ExecutionSnapshot `json:"latest_execution,omitempty"`
	Periodic            bool               `json:"periodic"`
	ReadinessGraceUntil string             `json:"readiness_grace_until,omitempty"`
}

type Recorder interface {
	RecordTaskSnapshot(ctx context.Context, snapshot Snapshot) error
}

type ExecutionSnapshot struct {
	ExecutionID string `json:"execution_id"`
	TenantID    int    `json:"tenant_id,omitempty"`
	TaskName    string `json:"task_name"`
	RunID       string `json:"run_id,omitempty"`
	Kind        string `json:"kind"`
	Status      string `json:"status"`
	StartedAt   string `json:"started_at"`
	StoppedAt   string `json:"stopped_at,omitempty"`
	Error       string `json:"error,omitempty"`
}

type ExecutionRecorder interface {
	RecordTaskExecution(ctx context.Context, snapshot ExecutionSnapshot) error
}

type runtimeContextKey string

const (
	runtimeTaskNameKey runtimeContextKey = "task_name"
	runtimeRunIDKey    runtimeContextKey = "run_id"
	runtimeRecorderKey runtimeContextKey = "recorder"
	runtimeSnapshotKey runtimeContextKey = "snapshot_updater"
)

const periodicReadinessFailureThreshold = 3

type Group struct {
	logger *slog.Logger

	mu       sync.Mutex
	tasks    []Task
	snapshot map[string]Snapshot
	recorder Recorder
	started  bool
	wait     sync.WaitGroup
	done     chan struct{}
}

var runIDCounter atomic.Uint64
var executionIDCounter atomic.Uint64

func New(logger *slog.Logger) *Group {
	if logger == nil {
		logger = slog.Default()
	}
	return &Group{logger: logger, snapshot: map[string]Snapshot{}, done: make(chan struct{})}
}

func (g *Group) WithRecorder(recorder Recorder) *Group {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.recorder = recorder
	return g
}

func Periodic(cfg PeriodicConfig, run func(context.Context) error) func(context.Context) error {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return func(ctx context.Context) error {
		if ctx == nil {
			ctx = context.Background()
		}
		if run == nil {
			return fmt.Errorf("periodic task %s run function is required", strings.TrimSpace(cfg.Name))
		}
		if cfg.Interval <= 0 {
			return fmt.Errorf("periodic task %s interval must be positive", strings.TrimSpace(cfg.Name))
		}
		startupGrace := periodicStartupGrace(cfg)
		updateRuntimeSnapshot(ctx, func(snapshot Snapshot) Snapshot {
			snapshot.ReadinessGraceUntil = time.Now().Add(startupGrace).Format(time.RFC3339)
			return snapshot
		})
		consecutiveFailures := 0
		lastFailureLog := time.Time{}
		runOnce := func() {
			startedAt := time.Now()
			taskName := periodicTaskName(ctx, cfg.Name)
			var err error
			func() {
				defer func() {
					if recovered := recover(); recovered != nil {
						err = fmt.Errorf("panic: %v", recovered)
					}
				}()
				err = run(ctx)
			}()
			stoppedAt := time.Now()
			if errors.Is(err, context.Canceled) {
				return
			}
			execution := ExecutionSnapshot{
				ExecutionID: latestPeriodicSuccessExecutionID(taskName),
				TaskName:    taskName,
				RunID:       taskRunID(ctx),
				Kind:        ExecutionKindPeriodicTick,
				Status:      StatusSucceeded,
				StartedAt:   startedAt.Format(time.RFC3339),
				StoppedAt:   stoppedAt.Format(time.RFC3339),
			}
			if err != nil {
				execution.ExecutionID = latestPeriodicFailureExecutionID(taskName)
				execution.Status = StatusFailed
				execution.Error = observability.SanitizeText(err.Error())
			}
			recordTaskExecution(ctx, logger, execution)
			if err != nil {
				consecutiveFailures++
				updatePeriodicSnapshot(ctx, execution, consecutiveFailures, false)
				if !cfg.SuppressOutcomeLogs && (lastFailureLog.IsZero() || stoppedAt.Sub(lastFailureLog) >= time.Minute) {
					logger.Error("周期任务持续失败并将自动重试；请按任务名和运行标识检查依赖，重复日志已限流",
						"event", "periodic_task_failed", "component", "taskrunner", "task_name", taskName,
						"run_id", taskRunID(ctx), "execution_id", execution.ExecutionID, "step", "tick", "result", "retrying", "error_code", "PERIODIC_TASK_FAILED",
						"retry_count", consecutiveFailures, "duration_ms", stoppedAt.Sub(startedAt).Milliseconds())
					lastFailureLog = stoppedAt
				}
				return
			}
			previousFailures := consecutiveFailures
			consecutiveFailures = 0
			updatePeriodicSnapshot(ctx, execution, 0, true)
			if previousFailures > 0 {
				if !cfg.SuppressOutcomeLogs {
					logger.Info("周期任务已从连续失败中恢复",
						"event", "periodic_task_recovered", "component", "taskrunner", "task_name", taskName,
						"run_id", taskRunID(ctx), "execution_id", execution.ExecutionID, "step", "tick", "result", "success", "retry_count", previousFailures,
						"duration_ms", stoppedAt.Sub(startedAt).Milliseconds())
				}
				lastFailureLog = time.Time{}
			}
			if !cfg.SuppressOutcomeLogs {
				logger.Debug("周期任务本次执行完成", "event", "periodic_task_completed", "component", "taskrunner", "task_name", taskName,
					"run_id", taskRunID(ctx), "execution_id", execution.ExecutionID, "step", "tick", "result", "success", "duration_ms", stoppedAt.Sub(startedAt).Milliseconds())
			}
		}
		if cfg.RunOnStart {
			runOnce()
		}
		ticker := time.NewTicker(cfg.Interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
				runOnce()
			}
		}
	}
}

// ConfiguredPeriodic preserves periodic readiness metadata before the task
// goroutine becomes observable as running. Use it when adding Periodic work to
// a Group; Periodic remains available for standalone execution and tests.
func ConfiguredPeriodic(cfg PeriodicConfig, run func(context.Context) error) Task {
	return Task{
		Run:                    Periodic(cfg, run),
		Periodic:               true,
		PeriodicReadinessGrace: periodicStartupGrace(cfg),
	}
}

func periodicStartupGrace(cfg PeriodicConfig) time.Duration {
	startupGrace := 30 * time.Second
	if !cfg.RunOnStart {
		startupGrace += cfg.Interval
	}
	return startupGrace
}

func (g *Group) Add(name string, runnable any) {
	g.mu.Lock()
	defer g.mu.Unlock()
	task := Task{Name: strings.TrimSpace(name)}
	switch value := runnable.(type) {
	case func(context.Context) error:
		task.Run = value
	case Task:
		task.Run = value.Run
		task.Periodic = value.Periodic
		task.PeriodicReadinessGrace = value.PeriodicReadinessGrace
	}
	g.tasks = append(g.tasks, task)
}

func (g *Group) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.started {
		return fmt.Errorf("task group already started")
	}
	names := map[string]struct{}{}
	for _, task := range g.tasks {
		if task.Name == "" {
			return fmt.Errorf("task name is required")
		}
		if task.Run == nil {
			return fmt.Errorf("task %s run function is required", task.Name)
		}
		if _, ok := names[task.Name]; ok {
			return fmt.Errorf("duplicate task name: %s", task.Name)
		}
		names[task.Name] = struct{}{}
		g.snapshot[task.Name] = Snapshot{Name: task.Name, Status: StatusPending}
	}
	g.started = true
	g.wait.Add(len(g.tasks))
	for _, task := range g.tasks {
		task := task
		go func() {
			defer g.wait.Done()
			g.run(ctx, task)
		}()
	}
	go func() {
		g.wait.Wait()
		close(g.done)
	}()
	return nil
}

func (g *Group) Wait(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-g.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (g *Group) Snapshots() []Snapshot {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make([]Snapshot, 0, len(g.snapshot))
	for _, snapshot := range g.snapshot {
		out = append(out, snapshot)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (g *Group) run(ctx context.Context, task Task) {
	startedAt := time.Now()
	runID := newRunID(task.Name, startedAt)
	g.set(task.Name, func(snapshot Snapshot) Snapshot {
		snapshot.RunID = runID
		snapshot.Status = StatusRunning
		snapshot.StartedAt = startedAt.Format(time.RFC3339)
		snapshot.StoppedAt = ""
		snapshot.Error = ""
		snapshot.Periodic = task.Periodic
		if task.Periodic {
			snapshot.ReadinessGraceUntil = startedAt.Add(task.PeriodicReadinessGrace).Format(time.RFC3339)
		}
		return snapshot
	})
	g.logger.Info("后台任务已启动", "event", "background_task_started", "component", "taskrunner", "task_name", task.Name, "run_id", runID, "step", "run", "result", "started")

	status := StatusStopped
	errText := ""
	defer func() {
		if recovered := recover(); recovered != nil {
			status = StatusFailed
			errText = observability.SanitizeText(fmt.Sprintf("panic: %v", recovered))
		}
		stoppedAt := time.Now()
		g.set(task.Name, func(snapshot Snapshot) Snapshot {
			snapshot.Status = status
			snapshot.StoppedAt = stoppedAt.Format(time.RFC3339)
			snapshot.Error = errText
			return snapshot
		})
		if errText != "" {
			g.logger.Error("后台任务异常停止；请按任务名和运行标识检查失败步骤",
				"event", "background_task_failed", "component", "taskrunner", "task_name", task.Name, "run_id", runID,
				"step", "run", "result", "failed", "error_code", "BACKGROUND_TASK_FAILED", "duration_ms", stoppedAt.Sub(startedAt).Milliseconds())
			return
		}
		g.logger.Info("后台任务已停止", "event", "background_task_stopped", "component", "taskrunner", "task_name", task.Name, "run_id", runID,
			"step", "run", "result", "stopped", "duration_ms", stoppedAt.Sub(startedAt).Milliseconds())
	}()

	taskCtx := WithTaskRuntime(ctx, task.Name, runID, g.runtimeRecorder())
	taskCtx = context.WithValue(taskCtx, runtimeSnapshotKey, func(update func(Snapshot) Snapshot) {
		g.set(task.Name, update)
	})
	if err := task.Run(taskCtx); err != nil && !errors.Is(err, context.Canceled) {
		status = StatusFailed
		errText = observability.SanitizeText(err.Error())
	}
}

// RequiredTasksReady applies the runtime health contract for tasks that are
// actually registered by the current role. One transient periodic failure is
// tolerated; three consecutive failures remove readiness until recovery.
func RequiredTasksReady(snapshots []Snapshot, now time.Time) bool {
	if len(snapshots) == 0 {
		return false
	}
	for _, snapshot := range snapshots {
		if snapshot.Status != StatusRunning || snapshot.ConsecutiveFailures >= periodicReadinessFailureThreshold {
			return false
		}
		if snapshot.LastSuccessAt == "" && snapshot.ReadinessGraceUntil != "" {
			graceUntil, err := time.Parse(time.RFC3339, snapshot.ReadinessGraceUntil)
			if err != nil || now.After(graceUntil) {
				return false
			}
		}
	}
	return true
}

func updateRuntimeSnapshot(ctx context.Context, update func(Snapshot) Snapshot) {
	if ctx == nil || update == nil {
		return
	}
	if updater, ok := ctx.Value(runtimeSnapshotKey).(func(func(Snapshot) Snapshot)); ok && updater != nil {
		updater(update)
	}
}

func updatePeriodicSnapshot(ctx context.Context, execution ExecutionSnapshot, consecutiveFailures int, succeeded bool) {
	updateRuntimeSnapshot(ctx, func(snapshot Snapshot) Snapshot {
		latest := execution
		snapshot.LatestExecution = &latest
		snapshot.ConsecutiveFailures = consecutiveFailures
		if succeeded {
			snapshot.LastSuccessAt = execution.StoppedAt
		}
		return snapshot
	})
}

func (g *Group) runtimeRecorder() Recorder {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.recorder
}

func (g *Group) set(name string, update func(Snapshot) Snapshot) {
	var snapshot Snapshot
	var recorder Recorder
	g.mu.Lock()
	snapshot = g.snapshot[name]
	if snapshot.Name == "" {
		snapshot.Name = name
	}
	snapshot = update(snapshot)
	g.snapshot[name] = snapshot
	recorder = g.recorder
	g.mu.Unlock()

	if recorder == nil {
		return
	}
	recordCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := recorder.RecordTaskSnapshot(recordCtx, snapshot); err != nil {
		g.logger.Warn("后台任务状态写入失败；任务仍继续运行，请检查数据库",
			"event", "task_snapshot_record_failed", "component", "taskrunner", "task_name", snapshot.Name,
			"run_id", snapshot.RunID, "step", "persist_snapshot", "result", "failed", "error_code", "TASK_SNAPSHOT_RECORD_FAILED")
	}
}

func newRunID(name string, startedAt time.Time) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "task"
	}
	replacer := strings.NewReplacer(" ", "-", "/", "-", "\\", "-", ":", "-", "\t", "-")
	name = replacer.Replace(strings.ToLower(name))
	return fmt.Sprintf("%s-%d-%d", name, startedAt.UnixNano(), runIDCounter.Add(1))
}

func newExecutionID(taskName string, kind string, startedAt time.Time) string {
	taskName = strings.TrimSpace(taskName)
	if taskName == "" {
		taskName = "task"
	}
	kind = strings.TrimSpace(kind)
	if kind == "" {
		kind = "execution"
	}
	replacer := strings.NewReplacer(" ", "-", "/", "-", "\\", "-", ":", "-", "\t", "-")
	taskName = replacer.Replace(strings.ToLower(taskName))
	kind = replacer.Replace(strings.ToLower(kind))
	return fmt.Sprintf("%s-%s-%d-%d", taskName, kind, startedAt.UnixNano(), executionIDCounter.Add(1))
}

func NewExecutionID(taskName string, kind string, startedAt time.Time) string {
	return newExecutionID(taskName, kind, startedAt)
}

func latestPeriodicSuccessExecutionID(taskName string) string {
	taskName = strings.TrimSpace(taskName)
	if taskName == "" {
		taskName = "task"
	}
	replacer := strings.NewReplacer(" ", "-", "/", "-", "\\", "-", ":", "-", "\t", "-")
	return replacer.Replace(strings.ToLower(taskName)) + "-periodic-latest-success"
}

func latestPeriodicFailureExecutionID(taskName string) string {
	taskName = strings.TrimSpace(taskName)
	if taskName == "" {
		taskName = "task"
	}
	replacer := strings.NewReplacer(" ", "-", "/", "-", "\\", "-", ":", "-", "\t", "-")
	return replacer.Replace(strings.ToLower(taskName)) + "-periodic-latest-failure"
}

func WithTaskRuntime(ctx context.Context, taskName string, runID string, recorder any) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = context.WithValue(ctx, runtimeTaskNameKey, strings.TrimSpace(taskName))
	ctx = context.WithValue(ctx, runtimeRunIDKey, strings.TrimSpace(runID))
	if recorder != nil {
		ctx = context.WithValue(ctx, runtimeRecorderKey, recorder)
	}
	return ctx
}

func withTaskRuntime(ctx context.Context, taskName string, runID string, recorder any) context.Context {
	return WithTaskRuntime(ctx, taskName, runID, recorder)
}

func periodicTaskName(ctx context.Context, fallback string) string {
	return RuntimeTaskName(ctx, fallback)
}

func RuntimeTaskName(ctx context.Context, fallback string) string {
	if ctx == nil {
		return strings.TrimSpace(fallback)
	}
	if value, ok := ctx.Value(runtimeTaskNameKey).(string); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return strings.TrimSpace(fallback)
}

func taskRunID(ctx context.Context) string {
	return RuntimeRunID(ctx)
}

func RuntimeRunID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if value, ok := ctx.Value(runtimeRunIDKey).(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func recordTaskExecution(ctx context.Context, logger *slog.Logger, snapshot ExecutionSnapshot) {
	RecordTaskExecution(ctx, logger, snapshot)
}

func RecordTaskExecution(ctx context.Context, logger any, snapshot ExecutionSnapshot) {
	if ctx == nil {
		return
	}
	recorder, ok := ctx.Value(runtimeRecorderKey).(ExecutionRecorder)
	if !ok || recorder == nil {
		return
	}
	recordCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := recorder.RecordTaskExecution(recordCtx, snapshot); err != nil {
		structuredLogger, ok := logger.(*slog.Logger)
		if !ok || structuredLogger == nil {
			structuredLogger = slog.Default()
		}
		structuredLogger.Warn("后台任务执行记录写入失败；任务仍继续运行，请检查数据库",
			"event", "task_execution_record_failed", "component", "taskrunner", "task_name", snapshot.TaskName,
			"run_id", snapshot.RunID, "step", "persist_execution", "result", "failed", "error_code", "TASK_EXECUTION_RECORD_FAILED")
	}
}
