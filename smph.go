package smph

import (
	"context"
	"errors"
)

var (
	ErrFailedAcquire  = errors.New("error: failed to acquire resources")
	ErrInvalidRelease = errors.New("error: invalid release() parameter")
)

type Semaphore struct {
	ctx      context.Context
	self     chan struct{}
	requests chan request
}

type request struct {
	ctx  context.Context
	n    int
	done chan bool
}

func New(ctx context.Context, n int) *Semaphore {
	s := make(chan struct{}, n)
	for i := 0; i < n; i++ {
		s <- struct{}{}
	}

	smph := &Semaphore{
		ctx:      ctx,
		self:     s,
		requests: make(chan request, 1),
	}

	go smph.processRequests()

	return smph
}

func (s *Semaphore) Capacity() int {
	return cap(s.self)
}

func (s *Semaphore) Count() int {
	return len(s.self)
}

func (s *Semaphore) Busy() int {
	return cap(s.self) - len(s.self)
}

func (s *Semaphore) Acquire(ctx context.Context, n int) bool {
	done := make(chan bool)
	rctx, cancel := context.WithCancel(ctx)
	r := request{rctx, n, done}
	select {
	// cancels acquisition
	case <-ctx.Done():
		cancel()
		return false
	case s.requests <- r:
		select {
		case result := <-r.done:
			close(r.done)
			return result
		case <-ctx.Done():
			return false
		}
	}
}

func (s *Semaphore) processRequests() {
	for {
		select {
		// cleans up closed semaphore
		case <-s.ctx.Done():
			close(s.self)
			close(s.requests)
			return
		case r := <-s.requests:
			r.done <- s.acquire(r.ctx, r.n)
		}
	}
}

func (s *Semaphore) acquire(ctx context.Context, n int) bool {
	// acquire resources until ctx cancellation
	acquired := 0
	success := func() bool {
		for {
			select {
			case <-ctx.Done():
				return false
			case <-s.self:
				acquired++
				if acquired == n {
					return true
				}
			default:
			}
		}
	}()

	if !success {
		// put reserved resources back into pool
		for i := 0; i < acquired; i++ {
			s.self <- struct{}{}
		}
		return false
	}
	return true
}

func (s *Semaphore) AcquireNow(n int) bool {
	if len(s.self) >= n {
		for i := 0; i < n; i++ {
			<-s.self
		}
		return true
	}
	return false
}

func (s *Semaphore) Release(n int) {
	if n < 0 || n > cap(s.self)-s.Busy() {
		panic(ErrInvalidRelease)
	}

	for i := 0; i < n; i++ {
		s.self <- struct{}{}
	}
}
