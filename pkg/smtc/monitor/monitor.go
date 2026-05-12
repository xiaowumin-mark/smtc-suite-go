//go:build windows && cgo

// Package monitor watches system-wide Windows SMTC media sessions.
package monitor

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/xiaowumin-mark/smtc-suite-go/internal/winrt"
	smtcvt "github.com/xiaowumin-mark/smtc-suite-go/internal/winrt/smtc"
	"github.com/xiaowumin-mark/smtc-suite-go/pkg/smtc"
)

// Monitor watches system-wide SMTC sessions.
type Monitor struct {
	mu       sync.Mutex
	closed   bool
	manager  unsafe.Pointer

	managerEvents chan ManagerEvent
	sessions      map[string]*sessionInfo
}

// Config configures the Monitor.
type Config struct {
	ManagerEventBuffer int
}

// New creates and starts a Monitor.
func New(cfg *Config) (*Monitor, error) {
	if cfg == nil {
		cfg = &Config{}
	}
	buf := cfg.ManagerEventBuffer
	if buf <= 0 {
		buf = 16
	}

	if err := winrt.InitMTA(); err != nil {
		return nil, fmt.Errorf("monitor: %w", err)
	}

	factory, err := winrt.GetActivationFactory(
		smtcvt.RuntimeClass_GlobalSystemMediaTransportControlsSessionManager,
		winrt.IID_IGSMTCSessionManagerStatics,
	)
	if err != nil {
		winrt.UninitMTA()
		return nil, fmt.Errorf("monitor: get activation factory: %w", err)
	}
	defer winrt.Release(factory)

	asyncPtr, err := winrt.VtableGetPtr(factory, smtcvt.Slot_ManagerStatics_RequestAsync)
	if err != nil {
		winrt.UninitMTA()
		return nil, fmt.Errorf("monitor: RequestAsync: %w", err)
	}

	asyncOp := winrt.NewAsyncOperation(asyncPtr)
	managerPtr, err := asyncOp.Wait()
	if err != nil {
		asyncOp.Release()
		winrt.UninitMTA()
		return nil, fmt.Errorf("monitor: wait for session manager: %w", err)
	}
	asyncOp.Release()

	m := &Monitor{
		manager:       managerPtr,
		managerEvents: make(chan ManagerEvent, buf),
		sessions:      make(map[string]*sessionInfo),
	}

	m.refreshSessions()
	return m, nil
}

// Sessions returns a snapshot of all current sessions.
func (m *Monitor) Sessions() []smtc.SessionInfo {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]smtc.SessionInfo, 0, len(m.sessions))
	for _, s := range m.sessions {
		result = append(result, s.info)
	}
	return result
}

// CurrentSession returns the current active session, or nil if none.
func (m *Monitor) CurrentSession() *smtc.SessionInfo {
	m.mu.Lock()
	defer m.mu.Unlock()

	currentPtr, err := winrt.VtableGetPtr(m.manager, smtcvt.Slot_Manager_GetCurrentSession)
	if err != nil || currentPtr == nil {
		return nil
	}
	defer winrt.Release(currentPtr)

	id, _ := getSourceAppUserModelID(currentPtr)
	if s, ok := m.sessions[id]; ok {
		return &s.info
	}
	return nil
}

// Events returns a read-only channel of manager-level events.
func (m *Monitor) Events() <-chan ManagerEvent {
	return m.managerEvents
}

// Close releases all resources.
func (m *Monitor) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil
	}
	m.closed = true
	close(m.managerEvents)

	if m.manager != nil {
		winrt.Release(m.manager)
		m.manager = nil
	}

	winrt.UninitMTA()
	return nil
}

// refreshSessions enumerates all sessions.
func (m *Monitor) refreshSessions() {
	sessionsPtr, err := winrt.VtableGetPtr(m.manager, smtcvt.Slot_Manager_GetSessions)
	if err != nil || sessionsPtr == nil {
		return
	}
	defer winrt.Release(sessionsPtr)

	// IVectorView<IGSMTCSession*> - get_Size at vtable slot 7
	count32, err := winrt.VtableGetU32(sessionsPtr, 7)
	if err != nil {
		return
	}
	count := min(int32(count32), 50)

	for i := int32(0); i < count; i++ {
		sessionPtr, err := winrt.VtableGetPtrWithArg(sessionsPtr, 6, uintptr(i))
		if err != nil || sessionPtr == nil {
			continue
		}

		id, _ := getSourceAppUserModelID(sessionPtr)
		if id == "" {
			winrt.Release(sessionPtr)
			continue
		}

		info := fetchSessionInfo(sessionPtr, id)
		m.sessions[id] = &sessionInfo{info: info}
		winrt.Release(sessionPtr)
	}
}

func getSourceAppUserModelID(sessionPtr unsafe.Pointer) (string, error) {
	hstr, err := winrt.VtableGetHSTRING(sessionPtr, smtcvt.Slot_Session_GetSourceAppUserModelId)
	if err != nil {
		return "", err
	}
	defer hstr.Delete()
	return hstr.String(), nil
}

func fetchSessionInfo(sessionPtr unsafe.Pointer, id string) smtc.SessionInfo {
	info := smtc.SessionInfo{
		SessionID:            id,
		SourceAppUserModelID: id,
	}

	// GetPlaybackInfo (slot 9)
	if playbackPtr, err := winrt.VtableGetPtr(sessionPtr, smtcvt.Slot_Session_GetPlaybackInfo); err == nil && playbackPtr != nil {
		defer winrt.Release(playbackPtr)
		if status, err := winrt.VtableGetI32(playbackPtr, smtcvt.Slot_PlaybackInfo_PlaybackStatus); err == nil {
			info.PlaybackStatus = smtc.PlaybackStatus(status)
		}
	}

	// GetTimelineProperties (slot 8)
	if timelinePtr, err := winrt.VtableGetPtr(sessionPtr, smtcvt.Slot_Session_GetTimelineProperties); err == nil && timelinePtr != nil {
		defer winrt.Release(timelinePtr)
		info.TimelineInfo = fetchTimelineInfo(timelinePtr)
	}

	// TryGetMediaPropertiesAsync (slot 7)
	if mediaAsyncPtr, err := winrt.VtableGetPtr(sessionPtr, smtcvt.Slot_Session_TryGetMediaPropertiesAsync); err == nil && mediaAsyncPtr != nil {
		asyncOp := winrt.NewAsyncOperation(mediaAsyncPtr)
		if mediaPtr, err := asyncOp.Wait(); err == nil && mediaPtr != nil {
			defer winrt.Release(mediaPtr)
			info.MediaInfo = fetchMediaInfo(mediaPtr)
		}
		asyncOp.Release()
	}

	return info
}

func fetchTimelineInfo(timelinePtr unsafe.Pointer) smtc.TimelineInfo {
	var tl smtc.TimelineInfo

	if ticks, err := winrt.VtableGetTicks(timelinePtr, smtcvt.Slot_Timeline_Position); err == nil {
		tl.Position = smtc.TicksToDuration(ticks)
	}
	if ticks, err := winrt.VtableGetTicks(timelinePtr, smtcvt.Slot_Timeline_StartTime); err == nil {
		tl.StartTime = smtc.TicksToDuration(ticks)
	}
	if ticks, err := winrt.VtableGetTicks(timelinePtr, smtcvt.Slot_Timeline_EndTime); err == nil {
		tl.EndTime = smtc.TicksToDuration(ticks)
	}
	if ticks, err := winrt.VtableGetTicks(timelinePtr, smtcvt.Slot_Timeline_MinSeekTime); err == nil {
		tl.MinSeekTime = smtc.TicksToDuration(ticks)
	}
	if ticks, err := winrt.VtableGetTicks(timelinePtr, smtcvt.Slot_Timeline_MaxSeekTime); err == nil {
		tl.MaxSeekTime = smtc.TicksToDuration(ticks)
	}

	return tl
}

func fetchMediaInfo(mediaPtr unsafe.Pointer) smtc.MediaInfo {
	var mi smtc.MediaInfo

	if hstr, err := winrt.VtableGetHSTRING(mediaPtr, smtcvt.Slot_MediaProps_Title); err == nil {
		mi.Title = hstr.String()
		hstr.Delete()
	}
	if hstr, err := winrt.VtableGetHSTRING(mediaPtr, smtcvt.Slot_MediaProps_Artist); err == nil {
		mi.Artist = hstr.String()
		hstr.Delete()
	}
	if hstr, err := winrt.VtableGetHSTRING(mediaPtr, smtcvt.Slot_MediaProps_AlbumTitle); err == nil {
		mi.AlbumTitle = hstr.String()
		hstr.Delete()
	}
	if hstr, err := winrt.VtableGetHSTRING(mediaPtr, smtcvt.Slot_MediaProps_AlbumArtist); err == nil {
		mi.AlbumArtist = hstr.String()
		hstr.Delete()
	}
	if n, err := winrt.VtableGetI32(mediaPtr, smtcvt.Slot_MediaProps_TrackNumber); err == nil {
		mi.TrackNumber = n
	}
	if n, err := winrt.VtableGetI32(mediaPtr, smtcvt.Slot_MediaProps_AlbumTrackCount); err == nil {
		mi.AlbumTrackCount = n
	}

	return mi
}

// ---- Event types ----

// ManagerEventType classifies manager-level events.
type ManagerEventType int

const (
	ManagerEventSessionsChanged      ManagerEventType = iota
	ManagerEventCurrentSessionChanged
)

// ManagerEvent is a union type for manager-level events.
type ManagerEvent struct {
	Type             ManagerEventType
	Sessions         []smtc.SessionInfo
	CurrentSessionID string
}

type sessionInfo struct {
	info smtc.SessionInfo
}
