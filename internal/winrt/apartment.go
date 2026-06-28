//go:build windows && cgo

package winrt

import (
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
)

// ErrApartmentClosed is returned when work is submitted after the apartment
// worker has started shutting down.
var ErrApartmentClosed = errors.New("winrt: apartment worker is closed")

type apartmentJob struct {
	fn  func() error
	err chan error
}

// MTAWorker owns one locked OS thread initialized for COM/WinRT MTA work.
type MTAWorker struct {
	jobs      chan apartmentJob
	closeCh   chan struct{}
	done      chan struct{}
	closeOnce sync.Once
	closed    atomic.Bool
}

// NewMTAWorker starts a locked OS thread and initializes COM/WinRT on it.
func NewMTAWorker() (*MTAWorker, error) {
	w := &MTAWorker{
		jobs:    make(chan apartmentJob),
		closeCh: make(chan struct{}),
		done:    make(chan struct{}),
	}
	ready := make(chan error, 1)
	go w.run(ready)
	if err := <-ready; err != nil {
		<-w.done
		return nil, err
	}
	return w, nil
}

func (w *MTAWorker) run(ready chan<- error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(w.done)

	if err := InitMTA(); err != nil {
		ready <- err
		return
	}
	ready <- nil
	defer UninitMTA()

	for {
		select {
		case job := <-w.jobs:
			job.err <- job.fn()
		case <-w.closeCh:
			return
		}
	}
}

// Do runs fn on the worker's COM-initialized OS thread.
func (w *MTAWorker) Do(fn func() error) error {
	if w == nil {
		return ErrApartmentClosed
	}
	if fn == nil {
		return nil
	}
	if w.closed.Load() {
		return ErrApartmentClosed
	}

	errCh := make(chan error, 1)
	job := apartmentJob{fn: fn, err: errCh}
	select {
	case w.jobs <- job:
	case <-w.done:
		return ErrApartmentClosed
	}

	return <-errCh
}

// Close stops the worker after any currently running job returns.
func (w *MTAWorker) Close() {
	if w == nil {
		return
	}
	w.closeOnce.Do(func() {
		w.closed.Store(true)
		close(w.closeCh)
		<-w.done
	})
}
