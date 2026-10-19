# Audiobook Player

A small desktop audiobook player for macOS and Windows (Linux works too), written in Go with [Fyne](https://fyne.io).

## Features

- Plays **MP3, WAV, FLAC and Ogg Vorbis**.
- **Multi-part books**: open a folder and its files play in order (`Part 2` before `Part 10`). The next part starts automatically, and ⏮ / ⏭ and a **Parts** list let you jump between parts.
- **Resumes** the last book, part and position when you reopen the app.
- **Speed from 0.5x to 2.0x** in 0.1x steps, without changing the pitch of the voice.
- Skip 30 seconds back or forward. The remaining time takes the playback speed into account.
- Shows cover art from the book's folder (`cover.jpg`, `folder.png`, an image with the same name as the audio file, or the first image found).
- Uses the native file picker on macOS and Windows. You can also drag a file or folder onto the window.

### Supported files

| Format | Notes |
|---|---|
| MP3 | |
| WAV | 8, 16 and 24-bit PCM (32-bit float is not supported) |
| FLAC | Any bit depth |
| Ogg Vorbis (`.ogg`, `.oga`) | Mono and stereo |

M4B/M4A (AAC), Opus and copy-protected Audible or Apple Books files are not supported.

### Keyboard shortcuts

| Key | Action |
|---|---|
| Space | Play / pause |
| ← / → | Back / forward 30 seconds |
| ↑ / ↓ | Volume up / down |

## Running from source

You need [Go 1.22 or newer](https://go.dev/dl/) and a C compiler, because Fyne uses OpenGL through cgo.

**macOS**

```sh
xcode-select --install
go run .
```

**Windows**

1. Install Go.
2. Install a C compiler, for example the MinGW-w64 toolchain from [MSYS2](https://www.msys2.org/). Follow Fyne's [Windows setup guide](https://docs.fyne.io/started/#windows) and make sure `gcc` is on your `PATH`.
3. Run:

```powershell
go run .
```

**Linux (Debian or Ubuntu)**

```sh
sudo apt install gcc libgl1-mesa-dev xorg-dev libasound2-dev
go run .
```

On Linux the native file picker needs `zenity` or `kdialog`. If neither is installed, the app uses Fyne's own file browser.

## Building an app to share

`go build .` produces a plain executable. To get a proper app with its name and icon (`Audiobook Player.app` on macOS, `Audiobook Player.exe` on Windows), use Fyne's packaging tool. It reads `FyneApp.toml` and `Icon.png`:

```sh
go install fyne.io/fyne/v2/cmd/fyne@v2.5.4
fyne package -os darwin     # run on a Mac
fyne package -os windows    # run on Windows
```

Each package must be built on its own platform. To build for Windows from a Mac, use [fyne-cross](https://github.com/fyne-io/fyne-cross), which requires Docker.

## Project layout

```
main.go          starts the app and embeds the icon
FyneApp.toml     app name, ID and version used by `fyne package`
audio/           playback: Player, decoders for each format, pitch-preserving speed changer
library/         books on disk: Book, folder scanning and sorting, cover art, display names
ui/              the window
  app.go           UI type, start-up
  layout.go        building the window
  playback.go      play / pause, skip, speed, progress
  book.go          loading parts, Parts list, automatic next part
  pickers.go       native and fallback file dialogs, drag and drop
  keys.go          keyboard shortcuts
  preferences.go   saving and resuming
  widgets.go       round buttons, square cover layout
```

`audio` and `library` don't depend on the UI, so they can be tested without opening a window.

## Tests

```sh
go test ./...
```

Small sample audio files for the decoder tests are in `audio/testdata` (see the README there for their sources).
