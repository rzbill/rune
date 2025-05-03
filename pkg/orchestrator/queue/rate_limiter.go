// Package queue provides worker queue functionality for the orchestrator
package queue

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiterType defines the type of rate limiter to use
type RateLimiterType string

const (
	// RateLimiterTypeDefault is the default rate limiter
	RateLimiterTypeDefault RateLimiterType = "default"
	// RateLimiterTypeAggressive is a rate limiter with minimal backoff
	RateLimiterTypeAggressive RateLimiterType = "aggressive"
	// RateLimiterTypeConservative is a rate limiter with longer backoffs
	RateLimiterTypeConservative RateLimiterType = "conservative"
)

// RateLimiter limits the rate of item processing
type RateLimiter interface {
	// When gets the duration to wait before processing the item
	When(item interface{}) time.Duration
	// NumRequeues returns the number of times an item has been requeued
	NumRequeues(item interface{}) int
	// Forget indicates that an item is done being retried
	Forget(item interface{})
	// TryAccept attempts to accept an item or returns false if limiter is full
	TryAccept(item interface{}) bool
}

// NewRateLimiter creates a rate limiter based on the specified type
func NewRateLimiter(limiterType RateLimiterType) RateLimiter {
	switch limiterType {
	case RateLimiterTypeAggressive:
		return NewAggressiveRateLimiter()
	case RateLimiterTypeConservative:
		return NewConservativeRateLimiter()
	default:
		return NewDefaultRateLimiter()
	}
}

// exponentialFailureRateLimiter implements a rate limiter that increases backoff exponentially
type exponentialFailureRateLimiter struct {
	failuresLock sync.Mutex
	failures     map[interface{}]int
	baseDelay    time.Duration
	maxDelay     time.Duration
}

// NewExponentialFailureRateLimiter creates a new exponential failure rate limiter
func NewExponentialFailureRateLimiter(baseDelay, maxDelay time.Duration) *exponentialFailureRateLimiter {
	return &exponentialFailureRateLimiter{
		failures:  make(map[interface{}]int),
		baseDelay: baseDelay,
		maxDelay:  maxDelay,
	}
}

// When returns the time to wait before processing item
func (r *exponentialFailureRateLimiter) When(item interface{}) time.Duration {
	r.failuresLock.Lock()
	defer r.failuresLock.Unlock()

	numFailures := r.failures[item]
	r.failures[item] = numFailures + 1

	backoff := r.baseDelay
	for i := 0; i < numFailures; i++ {
		if backoff >= r.maxDelay {
			return r.maxDelay
		}
		backoff = backoff * 2
	}

	if backoff > r.maxDelay {
		backoff = r.maxDelay
	}

	return backoff
}

// NumRequeues returns the number of times the item has been requeued
func (r *exponentialFailureRateLimiter) NumRequeues(item interface{}) int {
	r.failuresLock.Lock()
	defer r.failuresLock.Unlock()
	return r.failures[item]
}

// Forget indicates that an item is done being retried
func (r *exponentialFailureRateLimiter) Forget(item interface{}) {
	r.failuresLock.Lock()
	defer r.failuresLock.Unlock()
	delete(r.failures, item)
}

// TryAccept always returns true for this rate limiter
func (r *exponentialFailureRateLimiter) TryAccept(item interface{}) bool {
	return true
}

// bucketRateLimiter uses a token bucket to limit the rate of operations
type bucketRateLimiter struct {
	limiter *rate.Limiter
}

// NewBucketRateLimiter creates a new rate limiter that limits overall QPS
func NewBucketRateLimiter(qps rate.Limit, burst int) *bucketRateLimiter {
	return &bucketRateLimiter{
		limiter: rate.NewLimiter(qps, burst),
	}
}

// When returns the time to wait before processing item
func (r *bucketRateLimiter) When(item interface{}) time.Duration {
	return r.limiter.Reserve().Delay()
}

// NumRequeues always returns 0 for this rate limiter
func (r *bucketRateLimiter) NumRequeues(item interface{}) int {
	return 0
}

// Forget does nothing for this rate limiter
func (r *bucketRateLimiter) Forget(item interface{}) {}

// TryAccept tries to accept the item without waiting
func (r *bucketRateLimiter) TryAccept(item interface{}) bool {
	return r.limiter.Allow()
}

// maxRateLimiter uses the maximum value from multiple rate limiters
type maxRateLimiter struct {
	limiters []RateLimiter
}

// NewMaxRateLimiter creates a new rate limiter that takes the maximum wait time
// from all its constituent rate limiters
func NewMaxRateLimiter(limiters ...RateLimiter) *maxRateLimiter {
	return &maxRateLimiter{
		limiters: limiters,
	}
}

// When returns the maximum time to wait from all rate limiters
func (r *maxRateLimiter) When(item interface{}) time.Duration {
	var max time.Duration
	for _, limiter := range r.limiters {
		wait := limiter.When(item)
		if wait > max {
			max = wait
		}
	}
	return max
}

// NumRequeues returns the maximum number of requeues from all limiters
func (r *maxRateLimiter) NumRequeues(item interface{}) int {
	var max int
	for _, limiter := range r.limiters {
		reqCount := limiter.NumRequeues(item)
		if reqCount > max {
			max = reqCount
		}
	}
	return max
}

// Forget forgets the item in all limiters
func (r *maxRateLimiter) Forget(item interface{}) {
	for _, limiter := range r.limiters {
		limiter.Forget(item)
	}
}

// TryAccept returns false if any limiter returns false
func (r *maxRateLimiter) TryAccept(item interface{}) bool {
	for _, limiter := range r.limiters {
		if !limiter.TryAccept(item) {
			return false
		}
	}
	return true
}

// NewDefaultRateLimiter creates a rate limiter optimized for typical reconciliation
func NewDefaultRateLimiter() RateLimiter {
	return NewMaxRateLimiter(
		// Initial backoff of 1s, max of 5 minutes
		NewExponentialFailureRateLimiter(1*time.Second, 5*time.Minute),
		// Overall rate limit of 50 qps with burst of 200
		NewBucketRateLimiter(rate.Limit(50), 200),
	)
}

// NewAggressiveRateLimiter creates a rate limiter with minimal backoff
func NewAggressiveRateLimiter() RateLimiter {
	return NewMaxRateLimiter(
		// Short backoff: 200ms initial, 1 minute max
		NewExponentialFailureRateLimiter(200*time.Millisecond, 1*time.Minute),
		// High throughput: 100 qps with burst of 300
		NewBucketRateLimiter(rate.Limit(100), 300),
	)
}

// NewConservativeRateLimiter creates a rate limiter with longer backoffs
func NewConservativeRateLimiter() RateLimiter {
	return NewMaxRateLimiter(
		// Longer backoff: 2s initial, 10 minutes max
		NewExponentialFailureRateLimiter(2*time.Second, 10*time.Minute),
		// Lower throughput: 20 qps with burst of 50
		NewBucketRateLimiter(rate.Limit(20), 50),
	)
}
