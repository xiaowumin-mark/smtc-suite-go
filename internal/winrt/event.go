//go:build windows && cgo

package winrt

// #include "c/helpers.h"
import "C"
import (
	"runtime/cgo"
	"sync"
	"syscall"
	"unsafe"
)

// EventHandler wraps a WinRT ITypedEventHandler<TSender, TArgs> COM object
// bridging WinRT events to Go callbacks.
//
// WinRT events follow this pattern:
//   1. Event source has add_EventName(slot N) and remove_EventName(slot N+1) methods
//   2. Create an ITypedEventHandler COM object and pass it to add_EventName
//   3. When the event fires, WinRT calls Invoke(sender, args) on the handler
//   4. add_EventName returns an EventRegistrationToken for unsubscribe
//
// Thread safety: Invoke is called on arbitrary WinRT threads. Our implementation
// uses cgo.Handle to safely call back into Go from any C thread.
//
// Full implementation deferred to Phase 1 (Monitor) where concrete event types
// and their parameterized IIDs are known.

// EventHandler manages a WinRT event subscription.
type EventHandler struct {
	mu         sync.Mutex
	obj        unsafe.Pointer // C-allocated COM object (ITypedEventHandler)
	token      C.EventRegistrationToken
	source     unsafe.Pointer // event source COM object
	addSlot    int            // vtable slot for add_EventName
	removeSlot int            // vtable slot for remove_EventName
	registered bool
	handle     cgo.Handle // handle to Go callback
}

// NewEventHandler creates a new EventHandler.
// The callback is invoked on the WinRT event thread and should do minimal work.
func NewEventHandler(callback func(sender, args unsafe.Pointer)) *EventHandler {
	h := cgo.NewHandle(callback)
	return &EventHandler{
		handle: h,
	}
}

// Register subscribes the handler to an event on the source object.
func (h *EventHandler) Register(source unsafe.Pointer, addSlot, removeSlot int) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.registered || h.obj == nil {
		return nil // TODO: implement COM object allocation in Phase 1
	}

	h.source = source
	h.addSlot = addSlot
	h.removeSlot = removeSlot

	fn := vtableFn(source, addSlot)
	r1, _, _ := syscall.SyscallN(fn,
		uintptr(source),
		uintptr(h.obj),
		uintptr(unsafe.Pointer(&h.token)),
	)
	if int32(r1) < 0 {
		return hresultErrorInt("add_EventHandler", int32(r1))
	}

	h.registered = true
	return nil
}

// Unregister removes the event subscription.
func (h *EventHandler) Unregister() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.registered {
		return nil
	}

	fn := vtableFn(h.source, h.removeSlot)
	r1, _, _ := syscall.SyscallN(fn,
		uintptr(h.source),
		uintptr(unsafe.Pointer(&h.token)),
	)
	if int32(r1) < 0 {
		return hresultErrorInt("remove_EventHandler", int32(r1))
	}

	h.registered = false
	return nil
}

// Close releases the event handler and its resources.
func (h *EventHandler) Close() error {
	if err := h.Unregister(); err != nil {
		return err
	}
	if h.handle > 0 {
		h.handle.Delete()
	}
	return nil
}

// Token returns the EventRegistrationToken.
func (h *EventHandler) Token() C.EventRegistrationToken {
	return h.token
}
