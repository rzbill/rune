// Package queue provides worker queue functionality for the orchestrator
package queue

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rzbill/rune/pkg/log"
)

// WorkerQueueOptions contains configuration for a worker queue
type WorkerQueueOptions struct {
	// Name of the queue for metrics and logs
	Name string

	// Number of worker goroutines to run
	Workers int

	// Function to process queue items
	ProcessFunc func(key string) error

	// Logger for the queue
	Logger log.Logger

	// Type of rate limiter to use
	RateLimiterType RateLimiterType

	// Whether to collect metrics
	EnableMetrics bool
}

// WorkerQueue manages concurrent processing of reconciliation tasks
type WorkerQueue struct {
	// Name of the queue for logs and metrics
	name string

	// The actual work queue
	queue *workQueue

	// Number of worker goroutines to run
	workers int

	// Function to process work items
	processFunc func(key string) error

	// Logger
	logger log.Logger

	// For tracking active workers
	wg sync.WaitGroup

	// For shutting down
	ctx    context.Context
	cancel context.CancelFunc

	// Whether metrics are enabled
	metricsEnabled bool

	// To prevent multiple stops
	stopped      bool
	stoppedMutex sync.Mutex

	// For tracking queue statistics
	stats struct {
		processed      int
		errors         int
		timeouts       int
		lastProcessed  time.Time
		processingTime time.Duration
		mu             sync.Mutex
	}
}

// DefaultWorkerCount is the default number of workers
const DefaultWorkerCount = 5

// workQueue is an implementation of a rate-limited work queue
type workQueue struct {
	// The queue of keys to process
	items []interface{}
	// To protect queue operations
	mu sync.Mutex
	// Processing times for items
	processing map[interface{}]bool
	// For rate limiting
	rateLimiter RateLimiter
	// For delayed adds
	delayedItems map[interface{}]time.Time
	// For shutdown
	shutdown bool
	// For waiting
	cond *sync.Cond
	// For notifying workers of new items
	workCh chan struct{}
	// For delayed items
	timer *time.Timer
	// Flag to track if workCh has been closed
	workChClosed bool
}

// newWorkQueue creates a new work queue with rate limiting
func newWorkQueue(name string, rateLimiter RateLimiter) *workQueue {
	q := &workQueue{
		items:        make([]interface{}, 0),
		processing:   make(map[interface{}]bool),
		rateLimiter:  rateLimiter,
		delayedItems: make(map[interface{}]time.Time),
		workCh:       make(chan struct{}, 1),
		workChClosed: false,
	}
	q.cond = sync.NewCond(&q.mu)

	// Start background goroutine for delayed items
	go q.processDelayedItems()

	return q
}

// processDelayedItems processes items that were added with delay
func (q *workQueue) processDelayedItems() {
	for {
		// Check if we're shutting down
		q.mu.Lock()
		if q.shutdown {
			q.mu.Unlock()
			return
		}
		q.mu.Unlock()

		now := time.Now()

		// Find the next item to process
		q.mu.Lock()

		// Check if we're still running
		if q.shutdown {
			q.mu.Unlock()
			return
		}

		// Find the earliest deadline
		var earliest time.Time
		var nextItem interface{}

		for item, t := range q.delayedItems {
			if earliest.IsZero() || t.Before(earliest) {
				earliest = t
				nextItem = item
			}
		}

		q.mu.Unlock()

		// If there's nothing to do, wait for something to be added
		if earliest.IsZero() {
			// Use a select with a timeout so we can periodically check for shutdown
			select {
			case <-time.After(50 * time.Millisecond):
				// Check for shutdown on the next loop iteration
			}
			continue
		}

		// Wait until the earliest deadline
		waitTime := earliest.Sub(now)
		if waitTime > 0 {
			// Use a select with a timer so we can break out if shutdown occurs
			timer := time.NewTimer(waitTime)
			select {
			case <-timer.C:
				// Time to process the item
			}
			// Stop the timer to release resources
			if !timer.Stop() {
				// If the timer has already expired, drain the channel
				select {
				case <-timer.C:
				default:
				}
			}
		}

		// Check for shutdown one more time before adding the item
		q.mu.Lock()
		if q.shutdown {
			q.mu.Unlock()
			return
		}

		// Add the item to the queue
		if nextItem != nil {
			delete(q.delayedItems, nextItem)
			q.addNoLock(nextItem)
		}
		q.mu.Unlock()
	}
}

// Add adds an item to the queue
func (q *workQueue) Add(item interface{}) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.shutdown {
		return
	}

	q.addNoLock(item)
}

// addNoLock adds an item to the queue without locking
func (q *workQueue) addNoLock(item interface{}) {
	// If the item is already being processed, don't add it again
	if q.processing[item] {
		return
	}

	// Check if the item is already in the queue
	for _, queuedItem := range q.items {
		if queuedItem == item {
			return
		}
	}

	// Add the item to the queue
	q.items = append(q.items, item)

	// Notify workers that there's a new item
	select {
	case q.workCh <- struct{}{}:
	default:
		// Channel is full, which is fine - someone will get the message
	}

	// Signal any waiting goroutines
	q.cond.Signal()
}

// Len returns the current queue length
func (q *workQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

// Get gets an item from the queue to process
func (q *workQueue) Get() (item interface{}, shutdown bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// If we're shutting down, return with shutdown=true
	if q.shutdown {
		return nil, true
	}

	// Wait until there's an item to process
	for len(q.items) == 0 {
		q.cond.Wait()

		// Check for shutdown again after waking up
		if q.shutdown {
			return nil, true
		}
	}

	// Get the first item
	item = q.items[0]
	q.items = q.items[1:]

	// Mark the item as processing
	q.processing[item] = true

	return item, false
}

// Done marks an item as done processing
func (q *workQueue) Done(item interface{}) {
	q.mu.Lock()
	defer q.mu.Unlock()

	delete(q.processing, item)

	// Signal in case someone is waiting for an item to be processed
	q.cond.Broadcast()
}

// ShutDown shuts down the queue
func (q *workQueue) ShutDown() {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.shutdown = true

	// Clear the queue
	q.items = nil
	q.delayedItems = nil

	// Signal everyone waiting
	q.cond.Broadcast()

	// Close the notification channel only if it hasn't been closed already
	if !q.workChClosed {
		close(q.workCh)
		q.workChClosed = true
	}
}

// ShutDownWithDrain shuts down the queue after all items are processed
func (q *workQueue) ShutDownWithDrain() {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.shutdown = true

	// Wait until all items are processed
	for len(q.processing) > 0 {
		q.cond.Wait()
	}

	// Clear the queue
	q.items = nil
	q.delayedItems = nil

	// Signal everyone waiting
	q.cond.Broadcast()

	// Close the notification channel only if it hasn't been closed already
	if !q.workChClosed {
		close(q.workCh)
		q.workChClosed = true
	}
}

// AddAfter adds an item to the queue after the specified duration
func (q *workQueue) AddAfter(item interface{}, duration time.Duration) {
	if duration <= 0 {
		q.Add(item)
		return
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	if q.shutdown {
		return
	}

	q.delayedItems[item] = time.Now().Add(duration)
}

// AddRateLimited adds an item to the queue with rate limiting
func (q *workQueue) AddRateLimited(item interface{}) {
	q.AddAfter(item, q.rateLimiter.When(item))
}

// Forget removes an item from the rate limiter tracking
func (q *workQueue) Forget(item interface{}) {
	q.rateLimiter.Forget(item)
}

// NewWorkerQueue creates a new worker queue with the specified options
func NewWorkerQueue(options WorkerQueueOptions) *WorkerQueue {
	if options.Workers <= 0 {
		options.Workers = DefaultWorkerCount
	}

	if options.Name == "" {
		options.Name = "default"
	}

	if options.Logger == nil {
		options.Logger = log.GetDefaultLogger()
	}

	return &WorkerQueue{
		name:           options.Name,
		queue:          newWorkQueue(options.Name, NewRateLimiter(options.RateLimiterType)),
		workers:        options.Workers,
		processFunc:    options.ProcessFunc,
		logger:         options.Logger.WithComponent("worker-queue").WithField("queue", options.Name),
		metricsEnabled: options.EnableMetrics,
		stopped:        false,
	}
}

// Start launches the worker pool
func (wq *WorkerQueue) Start(ctx context.Context) error {
	wq.logger.Info("Starting worker queue",
		log.Int("workers", wq.workers),
		log.Str("name", wq.name))

	wq.ctx, wq.cancel = context.WithCancel(ctx)

	// Start worker goroutines
	for i := 0; i < wq.workers; i++ {
		wq.wg.Add(1)
		go func(workerID int) {
			defer wq.wg.Done()
			wq.logger.Debug("Starting worker", log.Int("workerID", workerID))
			wq.runWorker(workerID)
		}(i)
	}

	return nil
}

// Stop gracefully shuts down the worker pool
func (wq *WorkerQueue) Stop() {
	wq.stoppedMutex.Lock()
	if wq.stopped {
		wq.logger.Debug("Worker queue already stopped, ignoring Stop call", log.Str("name", wq.name))
		wq.stoppedMutex.Unlock()
		return
	}
	wq.stopped = true
	wq.stoppedMutex.Unlock()

	wq.logger.Info("Stopping worker queue", log.Str("name", wq.name))

	// Signal workers to stop by shutting down the queue
	wq.queue.ShutDown()

	// Cancel context to stop any ongoing operations
	if wq.cancel != nil {
		wq.cancel()
	}

	// Wait for all workers to finish with a timeout
	done := make(chan struct{})
	go func() {
		wq.wg.Wait()
		close(done)
	}()

	// Wait for workers to exit or timeout after 5 seconds
	select {
	case <-done:
		// All workers exited normally
	case <-time.After(5 * time.Second):
		wq.logger.Warn("Timed out waiting for workers to stop", log.Str("name", wq.name))
	}

	wq.logger.Info("Worker queue stopped", log.Str("name", wq.name))
}

// Enqueue adds a work item to the queue
func (wq *WorkerQueue) Enqueue(key string) {
	wq.queue.Add(key)
}

// EnqueueRateLimited adds a work item to the queue with rate limiting
func (wq *WorkerQueue) EnqueueRateLimited(key string) {
	wq.queue.AddRateLimited(key)
}

// EnqueueAfter adds a work item to the queue after a delay
func (wq *WorkerQueue) EnqueueAfter(key string, delay time.Duration) {
	wq.queue.AddAfter(key, delay)
}

// Len returns the current queue length
func (wq *WorkerQueue) Len() int {
	return wq.queue.Len()
}

// runWorker is the main processing loop for a single worker
func (wq *WorkerQueue) runWorker(workerID int) {
	for wq.processNextItem(workerID) {
		// Continue processing until there are no more items or shutdown
	}
}

// processNextItem processes the next work item and returns false when the queue is shut down
func (wq *WorkerQueue) processNextItem(workerID int) bool {
	// Get next work item from queue
	key, shutdown := wq.queue.Get()
	if shutdown {
		wq.logger.Debug("Worker shutting down", log.Int("workerID", workerID))
		return false
	}

	// Convert key to string
	keyStr, ok := key.(string)
	if !ok {
		wq.logger.Error("Expected string key",
			log.Int("workerID", workerID),
			log.Str("keyType", fmt.Sprintf("%T", key)))
		wq.queue.Done(key)
		return true
	}

	// Process the work item
	startTime := time.Now()
	err := func() error {
		// Mark item as done when we finish processing
		defer wq.queue.Done(key)

		// Process the work item
		_, cancel := context.WithTimeout(wq.ctx, 5*time.Minute)
		defer cancel()

		wq.logger.Debug("Processing item",
			log.Str("key", keyStr),
			log.Int("workerID", workerID))

		err := wq.processFunc(keyStr)

		// Update statistics
		wq.stats.mu.Lock()
		wq.stats.processed++
		wq.stats.lastProcessed = time.Now()
		processingTime := time.Since(startTime)
		wq.stats.processingTime += processingTime
		if err != nil {
			wq.stats.errors++
		}
		wq.stats.mu.Unlock()

		if err != nil {
			// Handle error (e.g., requeue with backoff)
			wq.queue.AddRateLimited(key)
			wq.logger.Error("Error processing item, requeuing",
				log.Str("key", keyStr),
				log.Int("workerID", workerID),
				log.Duration("processingTime", processingTime),
				log.Err(err))
		} else {
			// Reset rate limiting count on success
			wq.queue.Forget(key)
			wq.logger.Debug("Successfully processed item",
				log.Str("key", keyStr),
				log.Int("workerID", workerID),
				log.Duration("processingTime", processingTime))
		}

		return err
	}()

	// Track error but continue processing
	_ = err

	return true
}

// GetStats returns a copy of the queue statistics
func (wq *WorkerQueue) GetStats() (processed, errors, timeouts int, lastProcessed time.Time, avgProcessingTime time.Duration) {
	wq.stats.mu.Lock()
	defer wq.stats.mu.Unlock()

	var avg time.Duration
	if wq.stats.processed > 0 {
		avg = wq.stats.processingTime / time.Duration(wq.stats.processed)
	}

	return wq.stats.processed, wq.stats.errors, wq.stats.timeouts, wq.stats.lastProcessed, avg
}
