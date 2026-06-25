//go:build windows && cgo

package winrt

import (
	"sync/atomic"
	"testing"
	"unsafe"
)

func TestEventHandlerCloseKeepsExternalReferenceAlive(t *testing.T) {
	var calls int32
	h := NewTypedEventHandler([]*GUID{IID_ITypedEventHandler_GSMTCSessionManager_SessionsChangedEventArgs}, func(sender, args unsafe.Pointer) {
		atomic.AddInt32(&calls, 1)
	})
	if h.obj == nil {
		t.Fatal("event handler object was not allocated")
	}
	obj := uintptr(h.obj)

	ppv := queryEventInterface(t, obj, IID_ITypedEventHandler_GSMTCSessionManager_SessionsChangedEventArgs)
	if ppv != obj {
		t.Fatalf("QueryInterface returned %#x, want %#x", ppv, obj)
	}

	if err := h.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// The source can still touch a handler after Go has closed its subscription
	// if it holds a COM reference. This must not trip cgo.Handle.Value on a
	// deleted handle.
	ppv = queryEventInterface(t, obj, IID_IUnknown)
	if ppv != obj {
		t.Fatalf("QueryInterface after Close returned %#x, want %#x", ppv, obj)
	}
	eventRelease(ppv)

	eventInvoke(obj, 0, 0)
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("callback calls = %d, want 1", got)
	}

	eventRelease(obj)
}

func queryEventInterface(t *testing.T, obj uintptr, iid *GUID) uintptr {
	t.Helper()
	var ppv uintptr
	hr := eventQueryInterface(obj, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&ppv)))
	if hr != 0 {
		t.Fatalf("QueryInterface(%#x) HRESULT = 0x%08X", uintptr(unsafe.Pointer(iid)), uint32(hr))
	}
	if ppv == 0 {
		t.Fatal("QueryInterface returned nil object")
	}
	return ppv
}
