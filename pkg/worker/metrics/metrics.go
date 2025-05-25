package metrics

import (
	"sync/atomic"
	"time"
)

// Metrics tracks various metrics for the worker package
type Metrics struct {
	// Task metrics
	TasksSubmitted  atomic.Int64
	TasksCompleted  atomic.Int64
	TasksFailed     atomic.Int64
	TasksCancelled  atomic.Int64
	TasksInProgress atomic.Int64
	TasksQueued     atomic.Int64

	// Queue metrics
	QueueSize    atomic.Int64
	QueueLatency atomic.Int64 // in nanoseconds

	// Worker metrics
	ActiveWorkers     atomic.Int64
	IdleWorkers       atomic.Int64
	WorkerUtilization atomic.Uint64 // percentage * 100 (to avoid floating point)

	// Performance metrics
	AverageTaskTime atomic.Int64 // in nanoseconds
	TotalTaskTime   atomic.Int64 // in nanoseconds
	TaskCount       atomic.Int64
}

// NewMetrics creates a new metrics instance
func NewMetrics() *Metrics {
	return &Metrics{}
}

// RecordTaskSubmission records a task submission
func (m *Metrics) RecordTaskSubmission() {
	m.TasksSubmitted.Add(1)
	m.TasksQueued.Add(1)
}

// RecordTaskStart records the start of a task
func (m *Metrics) RecordTaskStart() {
	m.TasksQueued.Add(-1)
	m.TasksInProgress.Add(1)
}

// RecordTaskCompletion records the completion of a task
func (m *Metrics) RecordTaskCompletion(duration time.Duration) {
	m.TasksInProgress.Add(-1)
	m.TasksCompleted.Add(1)
	m.TotalTaskTime.Add(duration.Nanoseconds())
	m.TaskCount.Add(1)
	m.AverageTaskTime.Store(m.TotalTaskTime.Load() / m.TaskCount.Load())
}

// RecordTaskFailure records a task failure
func (m *Metrics) RecordTaskFailure() {
	m.TasksInProgress.Add(-1)
	m.TasksFailed.Add(1)
}

// RecordTaskCancellation records a task cancellation
func (m *Metrics) RecordTaskCancellation() {
	m.TasksInProgress.Add(-1)
	m.TasksCancelled.Add(1)
}

// UpdateQueueMetrics updates queue-related metrics
func (m *Metrics) UpdateQueueMetrics(size int64, latency time.Duration) {
	m.QueueSize.Store(size)
	m.QueueLatency.Store(latency.Nanoseconds())
}

// UpdateWorkerMetrics updates worker-related metrics
func (m *Metrics) UpdateWorkerMetrics(active, idle int64) {
	m.ActiveWorkers.Store(active)
	m.IdleWorkers.Store(idle)
	total := active + idle
	if total > 0 {
		utilization := uint64(float64(active) / float64(total) * 10000) // multiply by 10000 to store as integer
		m.WorkerUtilization.Store(utilization)
	}
}

// GetWorkerUtilization returns the worker utilization as a percentage
func (m *Metrics) GetWorkerUtilization() float64 {
	return float64(m.WorkerUtilization.Load()) / 100.0
}

// Reset resets all metrics to zero
func (m *Metrics) Reset() {
	m.TasksSubmitted.Store(0)
	m.TasksCompleted.Store(0)
	m.TasksFailed.Store(0)
	m.TasksCancelled.Store(0)
	m.TasksInProgress.Store(0)
	m.TasksQueued.Store(0)
	m.QueueSize.Store(0)
	m.QueueLatency.Store(0)
	m.ActiveWorkers.Store(0)
	m.IdleWorkers.Store(0)
	m.WorkerUtilization.Store(0)
	m.AverageTaskTime.Store(0)
	m.TotalTaskTime.Store(0)
	m.TaskCount.Store(0)
}
