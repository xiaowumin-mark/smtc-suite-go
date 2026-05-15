//go:build windows && cgo

// Example: List all SMTC sessions and print live Now Playing events.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/xiaowumin-mark/smtc-suite-go/pkg/smtc"
	"github.com/xiaowumin-mark/smtc-suite-go/pkg/smtc/monitor"
)

func main() {
	m, err := monitor.New(nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer m.Close()

	printSessions(m.Sessions())
	if cur := m.CurrentSession(); cur != nil {
		fmt.Printf("Current session: %s - %s\n", cur.MediaInfo.Title, cur.MediaInfo.Artist)
	}

	fmt.Println("Watching SMTC events for 60s. Start, pause, seek, or change tracks in a media app.")
	timer := time.NewTimer(60 * time.Second)
	defer timer.Stop()
	for {
		select {
		case evt, ok := <-m.Events():
			if !ok {
				return
			}
			printEvent(evt)
		case <-timer.C:
			return
		}
	}
}

func printSessions(sessions []smtc.SessionInfo) {
	if len(sessions) == 0 {
		fmt.Println("No active SMTC media sessions found.")
		fmt.Println("Open a media app like Spotify, Apple Music, or a browser playing media.")
		return
	}

	fmt.Printf("Found %d media session(s):\n\n", len(sessions))
	for i, s := range sessions {
		fmt.Printf("[%d] %s\n", i+1, s.SourceAppUserModelID)
		printSession("    ", s)
		fmt.Println()
	}
}

func printEvent(evt monitor.ManagerEvent) {
	switch evt.Type {
	case monitor.ManagerEventSessionsChanged:
		fmt.Printf("SessionsChanged: %d session(s)\n", len(evt.Sessions))
		for _, s := range evt.Sessions {
			fmt.Printf("  - %s: %s - %s (%s)\n", s.SourceAppUserModelID, s.MediaInfo.Title, s.MediaInfo.Artist, s.PlaybackStatus)
		}
	case monitor.ManagerEventCurrentSessionChanged:
		fmt.Printf("CurrentSessionChanged: %s\n", evt.CurrentSessionID)
	case monitor.ManagerEventSessionPlaybackChanged:
		fmt.Printf("SessionPlaybackChanged: %s\n", evt.SessionID)
		printEventSession(evt.Session)
	case monitor.ManagerEventSessionTimelineChanged:
		fmt.Printf("SessionTimelineChanged: %s\n", evt.SessionID)
		printEventSession(evt.Session)
	case monitor.ManagerEventSessionMediaChanged:
		fmt.Printf("SessionMediaChanged: %s\n", evt.SessionID)
		printEventSession(evt.Session)
	}
}

func printEventSession(s *smtc.SessionInfo) {
	if s == nil {
		return
	}
	printSession("  ", *s)
}

func printSession(prefix string, s smtc.SessionInfo) {
	fmt.Printf("%sTitle:    %s\n", prefix, s.MediaInfo.Title)
	fmt.Printf("%sArtist:   %s\n", prefix, s.MediaInfo.Artist)
	fmt.Printf("%sAlbum:    %s\n", prefix, s.MediaInfo.AlbumTitle)
	fmt.Printf("%sCover:    %t (%d bytes, sha256=%s)\n", prefix, s.MediaInfo.ThumbnailAvailable, len(s.MediaInfo.ThumbnailData), shortHash(s.MediaInfo.ThumbnailHash))
	fmt.Printf("%sStatus:   %s\n", prefix, s.PlaybackStatus)
	fmt.Printf("%sPosition: %v / %v\n", prefix, s.TimelineInfo.Position.Round(time.Second), s.TimelineInfo.EndTime.Round(time.Second))
}

func shortHash(hash string) string {
	if len(hash) <= 12 {
		return hash
	}
	return hash[:12]
}
