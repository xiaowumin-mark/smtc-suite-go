//go:build windows && cgo

package winrt

// #include "c/helpers.h"
import "C"
import (
	"fmt"
	"runtime/cgo"
	"sync"
	"sync/atomic"
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
// Thread safety: Invoke is called on arbitrary WinRT threads. The callback
// should do minimal work and hand off to a Go channel or goroutine quickly.

// EventHandler manages a WinRT event subscription.
type EventHandler struct {
	mu           sync.Mutex
	cond         *sync.Cond
	obj          unsafe.Pointer // C-allocated COM object (ITypedEventHandler)
	vtbl         unsafe.Pointer
	token        int64
	source       unsafe.Pointer // event source COM object
	addSlot      int            // vtable slot for add_EventName
	removeSlot   int            // vtable slot for remove_EventName
	registered   bool
	accepted     []*GUID
	objectHandle cgo.Handle // handle to EventHandler
	handle       cgo.Handle // handle to Go callback
	closed       bool
	inflight     int
}

var (
	_eventQICB      = syscall.NewCallback(eventQueryInterface)
	_eventAddRefCB  = syscall.NewCallback(eventAddRef)
	_eventReleaseCB = syscall.NewCallback(eventRelease)
	_eventInvokeCB  = syscall.NewCallback(eventInvoke)
)

const (
	eventCallbackHandleOffset = uintptr(8)
	eventObjectHandleOffset   = uintptr(16)
	eventRefCountOffset       = uintptr(24)
	eventCapturedIIDOffset    = uintptr(32)
	eventGUIDSize             = uintptr(16)
)

// NewEventHandler creates a new EventHandler.
// The callback is invoked on the WinRT event thread and should do minimal work.
func NewEventHandler(callback func(sender, args unsafe.Pointer)) *EventHandler {
	return NewTypedEventHandler(nil, callback)
}

// NewTypedEventHandler creates a new EventHandler that explicitly accepts typed
// event handler IIDs in QueryInterface.
func NewTypedEventHandler(accepted []*GUID, callback func(sender, args unsafe.Pointer)) *EventHandler {
	h := cgo.NewHandle(callback)
	ev := &EventHandler{
		accepted: accepted,
		handle:   h,
	}
	ev.cond = sync.NewCond(&ev.mu)
	ev.objectHandle = cgo.NewHandle(ev)
	ev.obj, ev.vtbl = newEventHandlerObject(uintptr(h), uintptr(ev.objectHandle))
	if ev.obj == nil {
		ev.objectHandle.Delete()
		ev.objectHandle = 0
		ev.handle.Delete()
		ev.handle = 0
	}
	return ev
}

// Register subscribes the handler to an event on the source object.
func (h *EventHandler) Register(source unsafe.Pointer, addSlot, removeSlot int) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return fmt.Errorf("winrt: event handler is closed")
	}
	if source == nil {
		return fmt.Errorf("winrt: event source is nil")
	}
	if h.registered {
		return nil
	}
	if h.obj == nil {
		return fmt.Errorf("winrt: event handler allocation failed")
	}

	fn := vtableFn(source, addSlot)
	var token int64
	r1, _, _ := syscall.SyscallN(fn,
		uintptr(source),
		uintptr(h.obj),
		uintptr(unsafe.Pointer(&token)),
	)
	if int32(r1) < 0 {
		return hresultErrorInt("add_EventHandler", int32(r1))
	}

	AddRef(source)
	h.source = source
	h.addSlot = addSlot
	h.removeSlot = removeSlot
	h.token = token
	h.registered = true
	return nil
}

// Unregister removes the event subscription.
func (h *EventHandler) Unregister() error {
	source, addSlot, removeSlot, token, ok := h.takeRegistration()
	if !ok {
		return nil
	}
	if err := removeEventRegistration(source, removeSlot, token); err != nil {
		h.restoreRegistration(source, addSlot, removeSlot, token)
		return err
	}
	Release(source)
	return nil
}

// Close releases the event handler and its resources.
func (h *EventHandler) Close() error {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return nil
	}
	h.closed = true
	source, removeSlot, token, registered := h.registrationLocked()
	h.registered = false
	h.source = nil
	h.addSlot = 0
	h.removeSlot = 0
	h.token = 0
	h.mu.Unlock()

	var err error
	if registered {
		err = removeEventRegistration(source, removeSlot, token)
		Release(source)
	}

	h.mu.Lock()
	for h.inflight > 0 {
		h.cond.Wait()
	}
	obj := h.obj
	h.obj = nil
	h.vtbl = nil
	h.mu.Unlock()

	if obj != nil {
		eventRelease(uintptr(obj))
	}
	return err
}

// Token returns the EventRegistrationToken.
func (h *EventHandler) Token() int64 {
	return h.token
}

func newEventHandlerObject(handle, objectHandle uintptr) (unsafe.Pointer, unsafe.Pointer) {
	objSize := eventCapturedIIDOffset + eventGUIDSize
	obj := heapAlloc(objSize)
	if obj == nil {
		return nil, nil
	}
	vtbl := heapAlloc(4 * unsafe.Sizeof(uintptr(0)))
	if vtbl == nil {
		heapFree(obj)
		return nil, nil
	}

	*(*uintptr)(unsafe.Add(vtbl, 0)) = _eventQICB
	*(*uintptr)(unsafe.Add(vtbl, 8)) = _eventAddRefCB
	*(*uintptr)(unsafe.Add(vtbl, 16)) = _eventReleaseCB
	*(*uintptr)(unsafe.Add(vtbl, 24)) = _eventInvokeCB

	*(*uintptr)(obj) = uintptr(vtbl)
	*(*uintptr)(unsafe.Add(obj, eventCallbackHandleOffset)) = handle
	*(*uintptr)(unsafe.Add(obj, eventObjectHandleOffset)) = objectHandle
	*(*int64)(unsafe.Add(obj, eventRefCountOffset)) = 1
	return obj, vtbl
}

func eventQueryInterface(self uintptr, riid uintptr, ppv uintptr) uintptr {
	if riid == 0 || ppv == 0 {
		return 0x80004003 // E_POINTER
	}
	g := (*GUID)(unsafe.Pointer(riid))
	if isSameGUID(g, IID_IUnknown) || isSameGUID(g, IID_IAgileObject) {
		return eventQueryInterfaceOK(self, ppv)
	}

	objectHandle := *(*uintptr)(unsafe.Pointer(self + eventObjectHandleOffset))
	if objectHandle != 0 {
		if h, ok := cgo.Handle(objectHandle).Value().(*EventHandler); ok {
			for _, accepted := range h.accepted {
				if isSameGUID(g, accepted) {
					return eventQueryInterfaceOK(self, ppv)
				}
			}
			if len(h.accepted) > 0 {
				*(*uintptr)(unsafe.Pointer(ppv)) = 0
				return 0x80004002 // E_NOINTERFACE
			}
		}
	}

	capturedPtr := (*GUID)(unsafe.Pointer(self + eventCapturedIIDOffset))
	if capturedPtr.Data1 == 0 && capturedPtr.Data2 == 0 && capturedPtr.Data3 == 0 {
		*capturedPtr = *g
	}
	if isSameGUID(g, capturedPtr) {
		return eventQueryInterfaceOK(self, ppv)
	}

	*(*uintptr)(unsafe.Pointer(ppv)) = 0
	return 0x80004002 // E_NOINTERFACE
}

func eventQueryInterfaceOK(self uintptr, ppv uintptr) uintptr {
	eventAddRef(self)
	*(*uintptr)(unsafe.Pointer(ppv)) = self
	return 0
}

func eventAddRef(self uintptr) uintptr {
	if self == 0 {
		return 0
	}
	return uintptr(atomic.AddInt64(eventRefCountPtr(self), 1))
}

func eventRelease(self uintptr) uintptr {
	if self == 0 {
		return 0
	}
	ref := atomic.AddInt64(eventRefCountPtr(self), -1)
	if ref == 0 {
		freeEventHandlerObject(self)
	}
	if ref < 0 {
		return 0
	}
	return uintptr(ref)
}

func eventInvoke(self uintptr, sender uintptr, args uintptr) uintptr {
	eventAddRef(self)
	defer eventRelease(self)

	objectHandle := *(*uintptr)(unsafe.Pointer(self + eventObjectHandleOffset))
	if objectHandle == 0 {
		return 0
	}
	h, ok := cgo.Handle(objectHandle).Value().(*EventHandler)
	if !ok || !h.beginInvoke() {
		return 0
	}
	defer h.endInvoke()

	handle := *(*uintptr)(unsafe.Pointer(self + eventCallbackHandleOffset))
	if handle == 0 {
		return 0
	}
	value := cgo.Handle(handle).Value()
	callback, ok := value.(func(sender, args unsafe.Pointer))
	if !ok || callback == nil {
		return 0
	}
	callback(unsafe.Pointer(sender), unsafe.Pointer(args))
	return 0
}

func (h *EventHandler) beginInvoke() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return false
	}
	h.inflight++
	return true
}

func (h *EventHandler) endInvoke() {
	h.mu.Lock()
	h.inflight--
	if h.inflight == 0 {
		h.cond.Broadcast()
	}
	h.mu.Unlock()
}

func (h *EventHandler) takeRegistration() (unsafe.Pointer, int, int, int64, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.registered {
		return nil, 0, 0, 0, false
	}
	source, removeSlot, token, ok := h.registrationLocked()
	addSlot := h.addSlot
	h.registered = false
	h.source = nil
	h.addSlot = 0
	h.removeSlot = 0
	h.token = 0
	return source, addSlot, removeSlot, token, ok
}

func (h *EventHandler) restoreRegistration(source unsafe.Pointer, addSlot, removeSlot int, token int64) {
	h.mu.Lock()
	if !h.closed {
		h.source = source
		h.addSlot = addSlot
		h.removeSlot = removeSlot
		h.token = token
		h.registered = true
	}
	h.mu.Unlock()
}

func (h *EventHandler) registrationLocked() (unsafe.Pointer, int, int64, bool) {
	if !h.registered || h.source == nil {
		return nil, 0, 0, false
	}
	return h.source, h.removeSlot, h.token, true
}

func removeEventRegistration(source unsafe.Pointer, removeSlot int, token int64) error {
	fn := vtableFn(source, removeSlot)
	r1, _, _ := syscall.SyscallN(fn,
		uintptr(source),
		uintptr(unsafe.Pointer(&token)),
	)
	if int32(r1) < 0 {
		return hresultErrorInt("remove_EventHandler", int32(r1))
	}
	return nil
}

func eventRefCountPtr(self uintptr) *int64 {
	return (*int64)(unsafe.Pointer(self + eventRefCountOffset))
}

func freeEventHandlerObject(self uintptr) {
	handle := *(*uintptr)(unsafe.Pointer(self + eventCallbackHandleOffset))
	if handle != 0 {
		cgo.Handle(handle).Delete()
		*(*uintptr)(unsafe.Pointer(self + eventCallbackHandleOffset)) = 0
	}

	objectHandle := *(*uintptr)(unsafe.Pointer(self + eventObjectHandleOffset))
	if objectHandle != 0 {
		cgo.Handle(objectHandle).Delete()
		*(*uintptr)(unsafe.Pointer(self + eventObjectHandleOffset)) = 0
	}

	vtbl := *(*uintptr)(unsafe.Pointer(self))
	if vtbl != 0 {
		*(*uintptr)(unsafe.Pointer(self)) = 0
		heapFree(unsafe.Pointer(vtbl))
	}
	heapFree(unsafe.Pointer(self))
}
