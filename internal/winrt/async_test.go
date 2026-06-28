//go:build windows && cgo

package winrt

import (
	"testing"
	"unsafe"
)

func TestCompletionHandlerRejectsIInspectable(t *testing.T) {
	h := newCompletionHandler()
	if h == nil || h.obj == 0 {
		t.Fatal("completion handler object was not allocated")
	}
	defer h.close()

	var ppv uintptr
	hr := handlerQueryInterface(h.obj, uintptr(unsafe.Pointer(IID_IInspectable)), uintptr(unsafe.Pointer(&ppv)))
	if uint32(hr) != 0x80004002 {
		t.Fatalf("QueryInterface(IInspectable) HRESULT = 0x%08X, want E_NOINTERFACE", uint32(hr))
	}
	if ppv != 0 {
		t.Fatalf("QueryInterface(IInspectable) returned %#x, want nil", ppv)
	}

	delegateIID := GUID{0x11111111, 0x2222, 0x3333, [8]byte{0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb}}
	hr = handlerQueryInterface(h.obj, uintptr(unsafe.Pointer(&delegateIID)), uintptr(unsafe.Pointer(&ppv)))
	if hr != 0 {
		t.Fatalf("QueryInterface(delegate) HRESULT = 0x%08X", uint32(hr))
	}
	if ppv != h.obj {
		t.Fatalf("QueryInterface(delegate) returned %#x, want %#x", ppv, h.obj)
	}
	handlerRelease(ppv)
}
