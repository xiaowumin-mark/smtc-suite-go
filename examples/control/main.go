//go:build windows && cgo

package main

import (
	"fmt"
	"os"

	"github.com/xiaowumin-mark/smtc-suite-go/pkg/smtc/control"
)

func main() {
	ctrl, err := control.New("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer ctrl.Close()

	fmt.Println("Testing control operations on current session...")
	if info, err := ctrl.MediaInfo(); err == nil {
		fmt.Printf("Current media: %s - %s (cover=%t, bytes=%d, sha256=%s)\n", info.Title, info.Artist, info.ThumbnailAvailable, len(info.ThumbnailData), shortHash(info.ThumbnailHash))
	}

	// Toggle play/pause
	fmt.Print("TogglePlayPause... ")
	if err := ctrl.TogglePlayPause(); err != nil {
		fmt.Printf("FAILED: %v\n", err)
	} else {
		fmt.Println("OK")
	}
}

func shortHash(hash string) string {
	if len(hash) <= 12 {
		return hash
	}
	return hash[:12]
}
