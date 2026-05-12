//go:build windows && cgo

// Example: List all SMTC sessions and their Now Playing info.
package main

import (
	"fmt"
	"os"

	"github.com/xiaowumin-mark/smtc-suite-go/pkg/smtc/monitor"
)

func main() {
	m, err := monitor.New(nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer m.Close()

	sessions := m.Sessions()
	if len(sessions) == 0 {
		fmt.Println("No active SMTC media sessions found.")
		fmt.Println("(Open a media app like Spotify, a browser playing YouTube, etc.)")
		return
	}

	fmt.Printf("Found %d media session(s):\n\n", len(sessions))
	for i, s := range sessions {
		fmt.Printf("[%d] %s\n", i+1, s.SourceAppUserModelID)
		fmt.Printf("    Title:   %s\n", s.MediaInfo.Title)
		fmt.Printf("    Artist:  %s\n", s.MediaInfo.Artist)
		fmt.Printf("    Album:   %s\n", s.MediaInfo.AlbumTitle)
		fmt.Printf("    Status:  %s\n", s.PlaybackStatus)
		fmt.Printf("    Position: %v / %v\n", s.TimelineInfo.Position, s.TimelineInfo.EndTime)
		fmt.Println()
	}

	// Show current session
	if cur := m.CurrentSession(); cur != nil {
		fmt.Printf("Current session: %s - %s\n", cur.MediaInfo.Title, cur.MediaInfo.Artist)
	}
}
