# smtc-suite-go

Go bindings for Windows System Media Transport Controls (SMTC), implemented with CGo and raw WinRT COM interop.

## Module Status

| Module | Status | Notes |
|---|---|---|
| `pkg/smtc/monitor` | Usable | Enumerates system media sessions, reads metadata/timeline/playback state, exposes manager and session events. |
| `pkg/smtc/control` | Usable | Controls existing media sessions with play/pause/skip/seek/shuffle/repeat. Playback rate needs a CGo ABI helper. |
| `pkg/smtc/create` | Experimental | Publishes a MediaPlayer-backed SMTC session with metadata, timeline, artwork, enabled buttons, and button events. |

## Requirements

| Requirement | Details |
|---|---|
| OS | Windows 10 build 17763+ |
| Go | 1.25+ |
| C compiler | MinGW-w64 (GCC) or MSVC-compatible CGo toolchain |
| Windows SDK | 10.0.17763+ |
| CGo | `CGO_ENABLED=1` |

Non-Windows and non-CGo builds expose stub packages that return `smtc.ErrUnsupported`.

## Quick Start

```go
package main

import (
    "fmt"

    "github.com/xiaowumin-mark/smtc-suite-go/pkg/smtc/monitor"
)

func main() {
    mgr, err := monitor.New(nil)
    if err != nil {
        panic(err)
    }
    defer mgr.Close()

    for _, s := range mgr.Sessions() {
        fmt.Println(s.SourceAppUserModelID, s.MediaInfo.Title, s.PlaybackStatus)
    }
}
```

## Examples

```powershell
go run ./examples/monitor
go run ./examples/control
go run ./examples/create
```

## Implementation Notes

- The project is Windows-only at runtime and uses raw COM vtable calls instead of a C++/WinRT bridge DLL.
- Monitor and control use MTA initialization; create uses the modern `Windows.Media.Playback.MediaPlayer` SMTC path.
- WinRT async operations are completed with `put_Completed` handlers and Go channels.
- Some advanced SMTC methods use floating-point parameters and require CGo wrappers to pass values through XMM registers on Windows x64.

## License

MIT
