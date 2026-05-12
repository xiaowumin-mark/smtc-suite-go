//go:build windows && cgo

package main

import (
	"fmt"

	"github.com/xiaowumin-mark/smtc-suite-go/internal/winrt"
	smtcvt "github.com/xiaowumin-mark/smtc-suite-go/internal/winrt/smtc"
)

func main() {
	fmt.Println("[1] Init MTA...")
	if err := winrt.InitMTA(); err != nil {
		fmt.Printf("    FAILED: %v\n", err)
		return
	}
	defer winrt.UninitMTA()

	fmt.Println("[2] Get SessionManager...")
	factory, _ := winrt.GetActivationFactory(
		smtcvt.RuntimeClass_GlobalSystemMediaTransportControlsSessionManager,
		winrt.IID_IGSMTCSessionManagerStatics,
	)
	asyncPtr, _ := winrt.VtableGetPtr(factory, smtcvt.Slot_ManagerStatics_RequestAsync)
	winrt.Release(factory)

	asyncOp := winrt.NewAsyncOperation(asyncPtr)
	managerPtr, _ := asyncOp.Wait()
	defer winrt.Release(managerPtr)
	defer asyncOp.Release()
	fmt.Printf("    OK manager=%p\n", managerPtr)

	fmt.Println("[3] GetSessions (manager slot 7)...")
	sessionsPtr, _ := winrt.VtableGetPtr(managerPtr, smtcvt.Slot_Manager_GetSessions)
	defer winrt.Release(sessionsPtr)

	// IVectorView: get_Size at slot 7, GetAt at slot 6
	count, _ := winrt.VtableGetU32(sessionsPtr, 7)
	fmt.Printf("    count=%d\n", count)

	fmt.Println("[4] Enumerate sessions...")
	for i := range min(int32(count), 10) {
		sessionPtr, err := winrt.VtableGetPtrWithArg(sessionsPtr, 6, uintptr(i))
		if err != nil || sessionPtr == nil {
			fmt.Printf("    [%d] failed: %v\n", i, err)
			continue
		}

		appID := "?"
		if hstr, err := winrt.VtableGetHSTRING(sessionPtr, smtcvt.Slot_Session_GetSourceAppUserModelId); err == nil {
			appID = hstr.String()
			hstr.Delete()
		}
		fmt.Printf("    [%d] %s\n", i, appID)

		// GetPlaybackInfo (slot 9)
		if piPtr, err := winrt.VtableGetPtr(sessionPtr, smtcvt.Slot_Session_GetPlaybackInfo); err == nil && piPtr != nil {
			status, _ := winrt.VtableGetI32(piPtr, smtcvt.Slot_PlaybackInfo_PlaybackStatus)
			fmt.Printf("         playbackStatus=%d\n", status)
			winrt.Release(piPtr)
		}

		// GetTimelineProperties (slot 8)
		if tlPtr, err := winrt.VtableGetPtr(sessionPtr, smtcvt.Slot_Session_GetTimelineProperties); err == nil && tlPtr != nil {
			pos, _ := winrt.VtableGetTicks(tlPtr, smtcvt.Slot_Timeline_Position)
			end, _ := winrt.VtableGetTicks(tlPtr, smtcvt.Slot_Timeline_EndTime)
			fmt.Printf("         position=%d end=%d\n", pos, end)
			winrt.Release(tlPtr)
		}

		winrt.Release(sessionPtr)
	}

	fmt.Println("\nDone!")
}
