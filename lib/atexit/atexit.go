// Package atexit provides handling for functions you want called when
// the program exits unexpectedly due to a signal.
//
// You should also make sure you call Run in the normal exit path.
package atexit

import (
	"os"
	"os/signal"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/rclone/rclone/fs"
)

var (
	fns          = make(map[FnHandle]uint64)
	fnsMutex     sync.Mutex
	fnSequence   uint64
	exitChan     chan os.Signal
	exitOnce     sync.Once
	registerOnce sync.Once
	signalled    atomic.Int32
	runCalled    atomic.Int32
)

// FnHandle is the type of the handle returned by function `Register`
// that can be used to unregister an at-exit function
type FnHandle *func()

// Register a function to be called on exit.
// Returns a handle which can be used to unregister the function with `Unregister`.
func Register(fn func()) FnHandle {
	if running() {
		return nil
	}
	fnsMutex.Lock()
	fnSequence++
	fns[&fn] = fnSequence
	fnsMutex.Unlock()

	// Run AtExit handlers on exitSignals so everything gets tidied up properly
	registerOnce.Do(func() {
		exitChan = make(chan os.Signal, 1)
		signal.Notify(exitChan, exitSignals...)
		go func() {
			sig := <-exitChan
			if sig == nil {
				return
			}
			signal.Stop(exitChan)
			signalled.Store(1)
			fs.Infof(nil, "Signal received: %s", sig)
			Run()
			fs.Infof(nil, "Exiting...")
			os.Exit(exitCode(sig))
		}()
	})

	return &fn
}

// Signalled returns true if an exit signal has been received
func Signalled() bool {
	return signalled.Load() != 0
}

// running returns true if run has been called
func running() bool {
	return runCalled.Load() != 0
}

// Unregister a function using the handle returned by `Register`
func Unregister(handle FnHandle) {
	if running() {
		return
	}
	fnsMutex.Lock()
	defer fnsMutex.Unlock()
	delete(fns, handle)
}

// IgnoreSignals disables the signal handler and prevents Run from being executed automatically
func IgnoreSignals() {
	if running() {
		return
	}
	registerOnce.Do(func() {})
	if exitChan != nil {
		signal.Stop(exitChan)
		close(exitChan)
		exitChan = nil
	}
}

func orderedFns(registrations map[FnHandle]uint64) []FnHandle {
	type registration struct {
		fn       FnHandle
		sequence uint64
	}
	ordered := make([]registration, 0, len(registrations))
	for fn, sequence := range registrations {
		ordered = append(ordered, registration{fn: fn, sequence: sequence})
	}
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].sequence > ordered[j].sequence
	})
	result := make([]FnHandle, len(ordered))
	for i, registration := range ordered {
		result[i] = registration.fn
	}
	return result
}

// Run all the at exit functions if they haven't been run already
func Run() {
	runCalled.Store(1)
	// Take the lock here (not inside the exitOnce) so we wait
	// until the exit handlers have run before any calls to Run()
	// return.
	fnsMutex.Lock()
	defer fnsMutex.Unlock()
	exitOnce.Do(func() {
		for _, fnHandle := range orderedFns(fns) {
			(*fnHandle)()
		}
	})
}

// OnError registers fn with atexit and returns a function which
// runs fn() if *perr != nil and deregisters fn
//
// It should be used in a defer statement normally so
//
//	defer OnError(&err, cancelFunc)()
//
// So cancelFunc will be run if the function exits with an error or
// at exit.
//
// cancelFunc will only be run once.
func OnError(perr *error, fn func()) func() {
	var once sync.Once
	onceFn := func() {
		once.Do(fn)
	}
	handle := Register(onceFn)
	return func() {
		defer Unregister(handle)
		if *perr != nil {
			onceFn()
		}
	}

}
