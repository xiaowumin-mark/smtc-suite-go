//go:build windows && cgo

package winrt

import (
	"unsafe"
)

// GUID represents a COM/WinRT interface identifier (16 bytes).
type GUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

// IID_IUnknown: {00000000-0000-0000-C000-000000000046}
var IID_IUnknown = &GUID{
	0x00000000, 0x0000, 0x0000,
	[8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46},
}

// IID_IInspectable: {AF86E2E0-B12D-4C6A-9C5A-D7AA65101E90}
var IID_IInspectable = &GUID{
	0xAF86E2E0, 0xB12D, 0x4C6A,
	[8]byte{0x9C, 0x5A, 0xD7, 0xAA, 0x65, 0x10, 0x1E, 0x90},
}

// IID_IAsyncInfo: {00000036-0000-0000-C000-000000000046}
var IID_IAsyncInfo = &GUID{
	0x00000036, 0x0000, 0x0000,
	[8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46},
}

// ---- Windows.Media.Control IIDs (from windows-rs authoritative source) ----

// IID_IGSMTCSessionManagerStatics: {2050C4EE-11A0-57DE-AED7-C97C70338245}
var IID_IGSMTCSessionManagerStatics = &GUID{
	0x2050C4EE, 0x11A0, 0x57DE,
	[8]byte{0xAE, 0xD7, 0xC9, 0x7C, 0x70, 0x33, 0x82, 0x45},
}

// IID_IGSMTCSessionManager: {CACE8EAC-E86E-504A-AB31-5FF8FF1BCE49}
var IID_IGSMTCSessionManager = &GUID{
	0xCACE8EAC, 0xE86E, 0x504A,
	[8]byte{0xAB, 0x31, 0x5F, 0xF8, 0xFF, 0x1B, 0xCE, 0x49},
}

// IID_IGSMTCSession: {7148C835-9B14-5AE2-AB85-DC9B1C14E1A8}
var IID_IGSMTCSession = &GUID{
	0x7148C835, 0x9B14, 0x5AE2,
	[8]byte{0xAB, 0x85, 0xDC, 0x9B, 0x1C, 0x14, 0xE1, 0xA8},
}

// IID_IGSMTCSessionMediaProperties: {68856CF6-ADB4-54B2-AC16-05837907ACB6}
var IID_IGSMTCSessionMediaProperties = &GUID{
	0x68856CF6, 0xADB4, 0x54B2,
	[8]byte{0xAC, 0x16, 0x05, 0x83, 0x79, 0x07, 0xAC, 0xB6},
}

// IID_IGSMTCSessionPlaybackInfo: {94B4B6CF-E8BA-51AD-87A7-C10ADE106127}
var IID_IGSMTCSessionPlaybackInfo = &GUID{
	0x94B4B6CF, 0xE8BA, 0x51AD,
	[8]byte{0x87, 0xA7, 0xC1, 0x0A, 0xDE, 0x10, 0x61, 0x27},
}

// IID_IGSMTCSessionPlaybackControls: {6501A3E6-BC7A-503A-BB1B-68F158F3FB03}
var IID_IGSMTCSessionPlaybackControls = &GUID{
	0x6501A3E6, 0xBC7A, 0x503A,
	[8]byte{0xBB, 0x1B, 0x68, 0xF1, 0x58, 0xF3, 0xFB, 0x03},
}

// IID_IGSMTCSessionTimelineProperties: {EDE34136-6F25-588D-8ECF-EA5B6735AAA5}
var IID_IGSMTCSessionTimelineProperties = &GUID{
	0xEDE34136, 0x6F25, 0x588D,
	[8]byte{0x8E, 0xCF, 0xEA, 0x5B, 0x67, 0x35, 0xAA, 0xA5},
}

// ---- Windows.Media IIDs (Create module) ----

// IID_ISystemMediaTransportControlsInterop: {ddb0472d-c911-4a1f-86d9-dc3d71a95f5a}
var IID_ISystemMediaTransportControlsInterop = &GUID{
	0xddb0472d, 0xc911, 0x4a1f,
	[8]byte{0x86, 0xd9, 0xdc, 0x3d, 0x71, 0xa9, 0x5f, 0x5a},
}

// ptr returns the GUID as an unsafe.Pointer for passing to COM methods.
func (g *GUID) ptr() unsafe.Pointer {
	if g == nil {
		return nil
	}
	return unsafe.Pointer(g)
}
