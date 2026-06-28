//go:build windows && cgo

package winrt

import (
	"sync/atomic"
	"testing"
	"time"
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
	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Fatalf("callback calls = %d, want no calls after Close", got)
	}

	eventRelease(obj)
}

func TestEventHandlerCloseWaitsForInflightCallback(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	var calls int32

	h := NewEventHandler(func(sender, args unsafe.Pointer) {
		close(entered)
		<-release
		atomic.AddInt32(&calls, 1)
	})
	if h.obj == nil {
		t.Fatal("event handler object was not allocated")
	}

	done := make(chan struct{})
	go func() {
		eventInvoke(uintptr(h.obj), 0, 0)
		close(done)
	}()
	<-entered

	closed := make(chan struct{})
	go func() {
		if err := h.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
		close(closed)
	}()

	select {
	case <-closed:
		t.Fatal("Close returned before the in-flight callback finished")
	case <-time.After(100 * time.Millisecond):
	}

	close(release)
	<-done
	<-closed

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("callback calls = %d, want 1", got)
	}
}

func TestEventHandlerRejectsIInspectable(t *testing.T) {
	h := NewEventHandler(func(sender, args unsafe.Pointer) {})
	if h.obj == nil {
		t.Fatal("event handler object was not allocated")
	}
	defer h.Close()

	var ppv uintptr
	hr := eventQueryInterface(uintptr(h.obj), uintptr(unsafe.Pointer(IID_IInspectable)), uintptr(unsafe.Pointer(&ppv)))
	if uint32(hr) != 0x80004002 {
		t.Fatalf("QueryInterface(IInspectable) HRESULT = 0x%08X, want E_NOINTERFACE", uint32(hr))
	}
	if ppv != 0 {
		t.Fatalf("QueryInterface(IInspectable) returned %#x, want nil", ppv)
	}

	ppv = queryEventInterface(t, uintptr(h.obj), IID_ITypedEventHandler_GSMTCSessionManager_SessionsChangedEventArgs)
	eventRelease(ppv)
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
