//go:build windows && cgo

package winrt

// #include "c/helpers.h"
import "C"
import (
	"fmt"
	"runtime"
	"sync"
)

// ---- MTA Initialization (for Monitor and Control modules) ----

var (
	mtaMu       sync.Mutex
	mtaRefCount int32 // number of active InitMTA calls (unpaired)
)

// InitMTA initializes COM/WinRT on the calling thread in MTA mode.
//
// Tries CoInitializeEx first (like go-libnp), then RoInitialize as fallback.
func InitMTA() error {
	mtaMu.Lock()
	defer mtaMu.Unlock()

	// Try classic COM init first (COINIT_MULTITHREADED = 0)
	hr := C.CoInitializeEx(nil, 0)
	if hr < 0 {
		// Fall back to RoInitialize
		hr = C.RoInitialize(C.RO_INIT_MULTITHREADED)
		if hr < 0 {
			return hresultError("COM/WinRT init (MTA)", hr)
		}
	}
	mtaRefCount++
	return nil
}

// UninitMTA uninitializes COM/WinRT on the calling thread.
func UninitMTA() {
	mtaMu.Lock()
	defer mtaMu.Unlock()

	if mtaRefCount > 0 {
		mtaRefCount--
	}
	C.CoUninitialize()
	C.RoUninitialize()
}

// ---- STA Initialization (for Create module) ----

// InitSTA initializes the Windows Runtime on the calling thread in
// Single-Threaded Apartment (STA) mode. The caller MUST lock the current
// goroutine to an OS thread via runtime.LockOSThread() before calling this.
//
// Each call to InitSTA must be paired with a call to UninitSTA.
func InitSTA() error {
	hr := C.RoInitialize(C.RO_INIT_SINGLETHREADED)
	if hr < 0 {
		return hresultError("RoInitialize(STA)", hr)
	}
	runtime.LockOSThread()
	return nil
}

// UninitSTA uninitializes the Windows Runtime (STA mode).
// The caller may then call runtime.UnlockOSThread().
func UninitSTA() {
	C.RoUninitialize()
}

// hresultError creates an error from an HRESULT value.
func hresultError(op string, hr C.HRESULT) error {
	return hresultErrorInt(op, int32(hr))
}

// hresultErrorInt creates an error from an HRESULT value (Go int32).
func hresultErrorInt(op string, hr int32) error {
	if hr >= 0 {
		return nil
	}
	return &HresultError{Op: op, Code: hr}
}

// HresultError represents a failed HRESULT from a COM/WinRT call.
type HresultError struct {
	Op   string
	Code int32
}

func (e *HresultError) Error() string {
	return fmt.Sprintf("winrt: %s: HRESULT 0x%08X", e.Op, uint32(e.Code))
}
