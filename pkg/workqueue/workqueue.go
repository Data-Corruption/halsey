// package workqueue provides a simple rate-limited job queue.
package workqueue

import (
	"math/rand"
	"sync"
	"time"

	"github.com/Data-Corruption/stdx/xlog"
)

type JobFunc func() error

type job struct {
	id string
	fn JobFunc
}

type Queue struct {
	mu       sync.Mutex
	cond     *sync.Cond
	jobs     []job
	inQueue  map[string]struct{}
	closed   bool
	interval time.Duration
	jitter   time.Duration
	log      *xlog.Logger

	wg        sync.WaitGroup
	runningID string
	running   bool

	// Backoff fields
	backoffBase    time.Duration
	backoffCurrent time.Duration
	backoffMax     time.Duration
}

// New creates and starts a queue.
// interval: minimum time between job executions.
// jitter: extra random delay in [0, jitter] added to each interval.
// backoff: initial backoff duration when a job fails. Doubles on each consecutive error, up to a max of 1 hour.
func New(log *xlog.Logger, interval, jitter, backoff time.Duration) *Queue {
	q := &Queue{
		jobs:           make([]job, 0),
		inQueue:        make(map[string]struct{}),
		interval:       interval,
		jitter:         jitter,
		log:            log,
		backoffBase:    backoff,
		backoffCurrent: backoff,
		backoffMax:     time.Hour,
	}
	q.cond = sync.NewCond(&q.mu)

	q.wg.Add(1)
	go q.loop()

	return q
}

// Enqueue adds a job by id.
// Returns false if the queue is closed or the id is already queued/running.
// If expedite is true, the job is inserted at the front of the queue.
func (q *Queue) Enqueue(id string, expedite bool, fn JobFunc) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return false
	}
	if _, exists := q.inQueue[id]; exists {
		return false
	}

	q.inQueue[id] = struct{}{}
	j := job{id: id, fn: fn}

	if expedite {
		q.jobs = append(q.jobs, job{}) // grow by 1
		copy(q.jobs[1:], q.jobs[:len(q.jobs)-1])
		q.jobs[0] = j
	} else {
		q.jobs = append(q.jobs, j)
	}

	q.cond.Signal()
	return true
}

// Has reports whether an id is either queued or currently running.
func (q *Queue) Has(id string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	_, ok := q.inQueue[id]
	return ok
}

// Len returns the number of queued (not running) jobs.
func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.jobs)
}

// ResetBackoff resets the backoff duration to its baseline value.
func (q *Queue) ResetBackoff() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.backoffCurrent = q.backoffBase
}

// Close stops accepting new jobs, drops any queued ones, and waits
// for the currently running job (if any) to finish.
// Cannot be called from within a job, will deadlock.
func (q *Queue) Close() {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		q.wg.Wait()
		return
	}
	q.closed = true

	// drop queued jobs and clean up inQueue.
	if len(q.jobs) > 0 {
		if q.running {
			// preserve the running ID in inQueue, drop the rest.
			for id := range q.inQueue {
				if id != q.runningID {
					delete(q.inQueue, id)
				}
			}
		} else {
			// no job currently running; nuke everything.
			for id := range q.inQueue {
				delete(q.inQueue, id)
			}
		}
		q.jobs = nil // or make([]job, 0)
	}

	q.cond.Broadcast()
	q.mu.Unlock()

	q.wg.Wait()
}

func (q *Queue) loop() {
	defer q.wg.Done()

	for {
		q.mu.Lock()
		for len(q.jobs) == 0 && !q.closed {
			q.cond.Wait()
		}
		// nothing queued and we're closed → done.
		if q.closed && len(q.jobs) == 0 && !q.running {
			q.mu.Unlock()
			return
		}

		j := q.jobs[0]
		q.jobs = q.jobs[1:]
		q.running = true
		q.runningID = j.id
		q.mu.Unlock()

		err := j.fn()
		if err != nil {
			q.log.Errorf("job %s failed: %v", j.id, err)

			// Apply backoff on error
			q.mu.Lock()
			backoffDuration := q.backoffCurrent
			// Double the backoff for next time, capped at max
			if q.backoffCurrent < q.backoffMax {
				q.backoffCurrent *= 2
				if q.backoffCurrent > q.backoffMax {
					q.backoffCurrent = q.backoffMax
				}
			}
			q.mu.Unlock()

			q.log.Warnf("backing off for %v due to job error", backoffDuration)
			time.Sleep(backoffDuration)
		} else {
			// Reset backoff on success
			q.mu.Lock()
			q.backoffCurrent = q.backoffBase
			q.mu.Unlock()
		}

		q.mu.Lock()
		delete(q.inQueue, j.id)
		q.running = false
		q.runningID = ""
		closed := q.closed
		empty := len(q.jobs) == 0
		q.mu.Unlock()

		// close was called and there are no more jobs queued:
		// exit immediately, no extra sleep.
		if closed && empty {
			return
		}

		sleep := q.interval
		if q.jitter > 0 {
			extra := time.Duration(rand.Int63n(int64(q.jitter)))
			sleep += extra
		}
		if sleep > 0 {
			time.Sleep(sleep)
		}
	}
}
