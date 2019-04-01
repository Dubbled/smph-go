package smph

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

var (
	testPool *Semaphore

	ErrCountMismatch         = errors.New("error: resource count mismatch")
	ErrExpectedFailure       = errors.New("error: operation expected to fail")
	ErrFailedReleaseRecovery = errors.New("error: failed to recover after releasing failure")
)

func TestMain(m *testing.M) {
	parentCtx, cancel := context.WithCancel(context.Background())
	testPool = New(parentCtx, 100)
	m.Run()
	cancel()
}

func testPoolCount(t *testing.T, before int) {
	fmt.Printf("before: %d\nafter: %d\n", before, testPool.Count())
	if before != testPool.Count() {
		t.Error(ErrCountMismatch)
	}
}

func TestAcquire(t *testing.T) {
	before := testPool.Count()
	num := 5
	if !testPool.Acquire(context.Background(), num) {
		t.Error(ErrFailedAcquire)
	}

	testPool.Release(num)
	testPoolCount(t, before)
}

func TestAcquireTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		time.Millisecond*50,
	)
	defer cancel()

	before := testPool.Count()
	num := testPool.Capacity() + 2
	if testPool.Acquire(ctx, num) {
		t.Error(ErrExpectedFailure)
	}
	time.Sleep(time.Microsecond * 100)
	testPoolCount(t, before)
}

func TestAcquireCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	before := testPool.Count()
	num := testPool.Capacity() + 5
	go func() {
		if testPool.Acquire(ctx, num) {
			t.Error(ErrExpectedFailure)
		}
	}()

	cancel()
	time.Sleep(time.Microsecond * 100) // avoid data race
	testPoolCount(t, before)
}

func TestAcquireNow(t *testing.T) {
	before := testPool.Count()
	num := 2
	if !testPool.AcquireNow(num) {
		t.Error(ErrFailedAcquire)
	}

	testPool.Release(num)
	testPoolCount(t, before)
}

func TestInvalidRelease(t *testing.T) {
	num := testPool.Capacity() + 10
	defer func() {
		if r := recover(); r == nil {
			t.Error(ErrFailedReleaseRecovery)
		}
	}() // recovery attempt

	go func() {
		if testPool.Acquire(context.Background(), num) {
			t.Error(ErrExpectedFailure)
		}
	}() // operation never finishes

	testPool.Release(num)
}
