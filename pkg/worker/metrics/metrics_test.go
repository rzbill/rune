package metrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMetrics(t *testing.T) {
	m := NewMetrics()

	// Test task metrics
	m.RecordTaskSubmission()
	assert.Equal(t, int64(1), m.TasksSubmitted.Load())
	assert.Equal(t, int64(1), m.TasksQueued.Load())

	m.RecordTaskStart()
	assert.Equal(t, int64(0), m.TasksQueued.Load())
	assert.Equal(t, int64(1), m.TasksInProgress.Load())

	m.RecordTaskCompletion(time.Second)
	assert.Equal(t, int64(0), m.TasksInProgress.Load())
	assert.Equal(t, int64(1), m.TasksCompleted.Load())
	assert.Equal(t, time.Second.Nanoseconds(), m.TotalTaskTime.Load())
	assert.Equal(t, time.Second.Nanoseconds(), m.AverageTaskTime.Load())

	m.RecordTaskFailure()
	assert.Equal(t, int64(1), m.TasksFailed.Load())

	m.RecordTaskCancellation()
	assert.Equal(t, int64(1), m.TasksCancelled.Load())

	// Test queue metrics
	m.UpdateQueueMetrics(10, time.Millisecond)
	assert.Equal(t, int64(10), m.QueueSize.Load())
	assert.Equal(t, time.Millisecond.Nanoseconds(), m.QueueLatency.Load())

	// Test worker metrics
	m.UpdateWorkerMetrics(5, 5)
	assert.Equal(t, int64(5), m.ActiveWorkers.Load())
	assert.Equal(t, int64(5), m.IdleWorkers.Load())
	assert.Equal(t, 50.0, m.GetWorkerUtilization())

	// Test reset
	m.Reset()
	assert.Equal(t, int64(0), m.TasksSubmitted.Load())
	assert.Equal(t, int64(0), m.TasksCompleted.Load())
	assert.Equal(t, int64(0), m.TasksFailed.Load())
	assert.Equal(t, int64(0), m.TasksCancelled.Load())
	assert.Equal(t, int64(0), m.TasksInProgress.Load())
	assert.Equal(t, int64(0), m.TasksQueued.Load())
	assert.Equal(t, int64(0), m.QueueSize.Load())
	assert.Equal(t, int64(0), m.QueueLatency.Load())
	assert.Equal(t, int64(0), m.ActiveWorkers.Load())
	assert.Equal(t, int64(0), m.IdleWorkers.Load())
	assert.Equal(t, 0.0, m.GetWorkerUtilization())
	assert.Equal(t, int64(0), m.AverageTaskTime.Load())
	assert.Equal(t, int64(0), m.TotalTaskTime.Load())
	assert.Equal(t, int64(0), m.TaskCount.Load())
}
