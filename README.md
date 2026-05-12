# smtc-suite-go

Go library for Windows System Media Transport Controls (SMTC) integration via CGo and WinRT COM interop.

## Features

- **Monitor** — Watch system-wide media sessions (track changes, playback status, timeline)
- **Control** — Control remote media sessions (play/pause, skip, seek, volume)
- **Create** — Publish your own media session to Windows system UI

## Requirements

| Requirement | Details |
|---|---|
| **OS** | Windows 10 build 17763+ |
| **Go** | 1.25+ |
| **C compiler** | MinGW-w64 (GCC) or MSVC |
| **Windows SDK** | 10.0.17763+ |
| **CGo** | `CGO_ENABLED=1` |

## Quick Start

```go
// Monitor system media sessions
mgr, _ := monitor.New(nil)
defer mgr.Close()

for _, s := range mgr.Sessions() {
    fmt.Println(s.MediaInfo.Title)
}
```

## License

MIT
