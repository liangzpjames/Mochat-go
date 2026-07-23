package taskrunner

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
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
	Name string
	Run  func(context.Context) error
}

type PeriodicConfig struct {
	Name       string
	Interval   time.Duration
	RunOnStart bool
	Logger     *log.Logger
}

type Snapshot struct {
	Name      string `json:"name"`
	RunID     string `json:"run_id,omitempty"`
	Status    string `json:"status"`
	StartedAt string `json:"started_at"`
	StoppedAt string `json:"stopped_at,omitempty"`
	Error     string `json:"error,omitempty"`
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
)

type Group struct {
	logger *log.Logger

	mu       sync.Mutex
	tasks    []Task
	snapshot map[string]Snapshot
	recorder Recorder
	started  bool
}

var runIDCounter atomic.Uint64
var executionIDCounter atomic.Uint64

func New(logger *log.Logger) *Group {
	if logger == nil {
		logger = log.Default()
	}
	return &Group{logger: logger, snapshot: map[string]Snapshot{}}
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
		logger = log.Default()
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
		runOnce := func() {
			startedAt := time.Now()
			taskName := periodicTaskName(ctx, cfg.Name)
			execution := ExecutionSnapshot{
				ExecutionID: newExecutionID(taskName, ExecutionKindPeriodicTick, startedAt),
				TaskName:    taskName,
				RunID:       taskRunID(ctx),
				Kind:        ExecutionKindPeriodicTick,
				Status:      StatusRunning,
				StartedAt:   startedAt.Format(time.RFC3339),
			}
			recordTaskExecution(ctx, logger, execution)
			err := run(ctx)
			stoppedAt := time.Now()
			execution.StoppedAt = stoppedAt.Format(time.RFC3339)
			execution.Status = StatusSucceeded
			if err != nil {
				if errors.Is(err, context.Canceled) {
					execution.Status = StatusStopped
				} else {
					execution.Status = StatusFailed
					execution.Error = err.Error()
				}
			}
			recordTaskExecution(ctx, logger, execution)
			if err != nil && !errors.Is(err, context.Canceled) {
				logger.Printf("periodic task failed: %s err=%v", strings.TrimSpace(cfg.Name), err)
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

func (g *Group) Add(name string, run func(context.Context) error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.tasks = append(g.tasks, Task{Name: strings.TrimSpace(name), Run: run})
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
	for _, task := range g.tasks {
		task := task
		go g.run(ctx, task)
	}
	return nil
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
		return snapshot
	})
	g.logger.Printf("background task started: %s", task.Name)

	status := StatusStopped
	errText := ""
	defer func() {
		if recovered := recover(); recovered != nil {
			status = StatusFailed
			errText = fmt.Sprintf("panic: %v", recovered)
		}
		stoppedAt := time.Now()
		g.set(task.Name, func(snapshot Snapshot) Snapshot {
			snapshot.Status = status
			snapshot.StoppedAt = stoppedAt.Format(time.RFC3339)
			snapshot.Error = errText
			return snapshot
		})
		if errText != "" {
			g.logger.Printf("background task failed: %s err=%s", task.Name, errText)
			return
		}
		g.logger.Printf("background task stopped: %s", task.Name)
	}()

	taskCtx := WithTaskRuntime(ctx, task.Name, runID, g.runtimeRecorder())
	if err := task.Run(taskCtx); err != nil && !errors.Is(err, context.Canceled) {
		status = StatusFailed
		errText = err.Error()
	}
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
		g.logger.Printf("record background task snapshot failed: %s status=%s err=%v", snapshot.Name, snapshot.Status, err)
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

func recordTaskExecution(ctx context.Context, logger *log.Logger, snapshot ExecutionSnapshot) {
	RecordTaskExecution(ctx, logger, snapshot)
}

func RecordTaskExecution(ctx context.Context, logger *log.Logger, snapshot ExecutionSnapshot) {
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
		if logger == nil {
			logger = log.Default()
		}
		logger.Printf("record background task execution failed: %s kind=%s status=%s err=%v", snapshot.TaskName, snapshot.Kind, snapshot.Status, err)
	}
}
