# Audiobook Player

A small desktop audiobook player for macOS and Windows (Linux works too), written in Go with [Fyne](https://fyne.io).

<p align="center">
  <img src="docs/screenshots/now-playing-light.png" width="380" alt="Now Playing in the light theme: the cover of Frankenstein, the title, Part 11 of 24 · Chapter 11, a progress bar at 3:35:00 with 5:00:59 left, and the play, skip and volume controls">
  <img src="docs/screenshots/now-playing-dark.png" width="380" alt="The same Now Playing screen in the dark theme">
</p>

## Features

- Plays **MP3, M4B, M4A, WAV, FLAC and Ogg Vorbis**.
- A **Library** of your audiobooks with cover art and how far you've got in each one, so you can switch books with one click. See [Library](#library).
- **Multi-part books**: open a folder and its files play in order (`Part 2` before `Part 10`). The next part starts automatically, and ⏮ / ⏭ and a **Parts** list let you jump between parts. Once the length of every part is known, the progress bar, times and 30-second skips cover the whole book.
- **Chapters in M4B and M4A files**: ⏮ / ⏭ jump between chapters, a **Chapters** list shows each chapter with its start time, and the current chapter's title is shown while you listen.
- **Remembers your place in every book**, and resumes the last one when you reopen the app.
- Shows the book title, author and part names from the files' tags (ID3, FLAC and Vorbis comments), falling back to file and folder names.
- **Speed from 0.5x to 2.0x** in 0.1x steps, without changing the pitch of the voice.
- Skip 30 seconds back or forward. The remaining time takes the playback speed into account.
- **Sleep timer**: the moon button next to the speed stops playback after 5 to 90 minutes, at the **end of the chapter** (books with chapters) or at the **end of this part** (books of several parts). It shows the time left, taking the playback speed into account, only counts down while a book is playing, and fades the volume out over the last 10 seconds before pausing. Stopping at the end of a chapter or part leaves the book at the start of the next one, so pressing play carries straight on. If you skip into another chapter, the timer stops at the end of that one instead.
- Shows cover art: an image with the same name as the audio file, else artwork embedded in the file, else `cover`, `folder` or `front` `.jpg`/`.png`, else the first image in the folder.
- A warm light and dark look. It follows your system setting, or you can choose Light or Dark in **Settings**. The Now Playing screen is tinted with the colours of the book's cover. On Windows the title bar turns light or dark to match.
- **Statistics**: total listening time, this week, daily average, books finished, listening streaks and average speed, with daily and weekly charts, a listening calendar and your most-listened books.
- **Collections** of your own, such as Currently reading, Classics or For bedtime. See [Collections](#collections).
- **Library folder**: choose a folder in **Settings** and the audiobooks in it are added to the library automatically, including ones you copy in later.
- **Media keys and system controls**: the play/pause and skip keys on your keyboard, headset buttons, and the media controls in Windows (taskbar, lock screen, the volume pop-up) and macOS (Control Center, the lock screen, AirPods) control the player and show the book's title and cover. See [Media keys](#media-keys-and-system-controls).
- Uses the native file picker on macOS and Windows. You can also drag a file or folder onto the window.

### Supported files

| Format | Notes |
|---|---|
| MP3 | |
| M4B, M4A | AAC-LC, mono and stereo (HE-AAC is not supported) |
| WAV | 8, 16 and 24-bit PCM (32-bit float is not supported) |
| FLAC | Any bit depth |
| Ogg Vorbis (`.ogg`, `.oga`) | Mono and stereo |

HE-AAC, Opus and copy-protected Audible (`.aa`, `.aax`) or Apple Books files are not supported.

Chapters are read from M4B and M4A files, from either a QuickTime chapter track (what Apple's tools and most converters write) or a Nero `chpl` box. ⏮ goes back to the start of the current chapter if you are more than 3 seconds into it, and to the previous chapter otherwise. Chapters in MP3 files (ID3 `CHAP` frames) are not read.

### Library

The **Library** tab shows every book you've added as a card with its cover, title, author and progress (for example `43% · 5h 12m left`, or `Finished`). The books you listened to most recently come first, unless you choose another order.

<p align="center">
  <img src="docs/screenshots/library-light.png" width="380" alt="The Library tab showing All books: a search box, Add books, the All, In progress, Not started and Finished filters, Sort: Recently played, and cards for Frankenstein, Pride and Prejudice, Moby-Dick and other books showing their author and how far through each one you are">
  <img src="docs/screenshots/collection-light.png" width="380" alt="The Library tab with the For bedtime collection open, showing its four books: The Time Machine, Treasure Island, The Adventures of Sherlock Holmes and Little Women">
</p>

- **Add books** lets you choose one or more folders. Every audiobook inside them is added, however deeply it is nested:
  - a folder of audio files is one book, and its files are the book's parts;
  - `.m4b` files are always books of their own, so a folder of `.m4b` files gives one book per file.
- You can also drop folders or files onto the Library tab.
- Any book you open with **Open file** or **Open folder** is added automatically.
- Click a book to carry on from where you stopped. Right-click a book to remove it from the library; this doesn't touch the files.
- Type in the search box (or press ⌘F on a Mac, Ctrl+F elsewhere) to find a book by its title, author, or file or folder name. Upper and lower case and accents don't matter, so `bronte` finds *Brontë*, and every word you type has to match.
- **All**, **In progress**, **Not started** and **Finished** above the books show only the books in that state. They work together with the search box, so **Finished** plus `tolkien` shows the Tolkien books you've finished.
- **Sort** orders the books by:
  - **Recently played**: the last book you listened to first;
  - **Title**: A to Z, ignoring case and accents, with `Book 2` before `Book 10`;
  - **Author**: A to Z by author, then by title. Books without an author come last;
  - **Progress**: books in progress first, furthest along first, then books you haven't started, then finished ones.

  The order you choose is remembered. The filter always starts at **All**, so books never seem to be missing when you open the app.
- A book whose files have been moved, or that is on a drive that isn't connected, shows `Not found`.

#### Collections

Collections are your own groups of books, such as **Currently reading**, **Classics** or **For bedtime**. A book can be in as many collections as you like, and its files stay where they are.

- The button at the top left of the Library shows where you are: **All books** or a collection. Its menu switches between them, makes a **New collection…**, and renames or deletes the collection you're in. Deleting a collection keeps its books in your library.
- Right-click a book and choose **Add to collection** to tick or untick the collections it's in, or to make a new one with it in. Inside a collection, **Remove from "Classics"** takes it out of just that collection.
- **Add books**, and dropping folders onto the Library, add the new books to the collection that's open.
- Search, the **In progress / Not started / Finished** filters and **Sort** all work inside a collection.
- Collections are saved in `library.json`, follow books that move, and drop books that have been deleted from the disk.

### Statistics

The **Statistics** tab shows how much you listen. Time is only counted while a book is actually playing, so it starts from when you first use this version.

<p align="center">
  <img src="docs/screenshots/statistics-dark.png" width="520" alt="The Statistics tab in the dark theme: tiles for total listening, this week, daily average, books finished, current streak and average speed, then bar charts of daily and weekly listening">
</p>

- **Total listening**, **This week** (since Monday) and **Daily average** (over the last 30 days) are real time spent listening, not book time, so listening at 1.5× for an hour counts as an hour.
- **Books finished** counts each book you have listened to the end, once.
- **Current streak** is the number of days in a row you have listened for at least a minute. Today only breaks the streak once it is over.
- **Average speed** is weighted by how long you listened at each speed.
- **Daily activity** (30 days) and **Weekly activity** (12 weeks) are bar charts; hover over a bar to see its value. The **Listening calendar** shows the last 26 weeks, one square per day, darker for more listening (lighter in dark mode).
- Every chart has a **Table** button that shows the same numbers as a list.

#### Library folder

In **Settings → Library folder** you can choose the folder where you keep your audiobooks. Every book in it is added to the library, using the same rules as **Add books**, and the folder is checked again:

- when the app starts and when you choose the folder,
- when you open the Library tab (at most every 30 seconds),
- every 2 minutes while the app is open, and
- when you click **Check now**.

A book whose files changed in the last 30 seconds is left until the next check, so a book that is still being copied isn't added half-finished. If a book gets a new part, its card is updated. Books you remove from the library aren't added again; adding one by hand brings it back. **Stop importing** turns this off and leaves the books already in your library where they are.

Progress needs the length of every part. For MP3 books this means reading each file once, which happens in the background the first time a book is added. The lengths are remembered after that.

### Your data

Everything the app remembers stays on your computer, in the app's folder:

| System | Folder |
|---|---|
| Windows | `%APPDATA%\fyne\com.izzzicos.audiobookplayer` |
| macOS | `~/Library/Preferences/fyne/com.izzzicos.audiobookplayer` |
| Linux | `~/.config/fyne/com.izzzicos.audiobookplayer` |

- `library.json` holds your library, where you are in each book and your listening statistics. It is saved by writing a complete new copy and then swapping it in, so a crash or power cut while saving can't leave a half-written file.
- `backups/` keeps a copy of `library.json` from each of the last 7 days. If `library.json` is ever unreadable, the app restores the newest good backup and tells you. If there is no backup, it keeps the damaged file aside instead of overwriting it.
- `preferences.json` holds your settings (volume, speed, appearance, library folder).

`library.json` doesn't grow forever. Each time the app starts it tidies up:

- **Part lengths** are forgotten for files that were deleted or changed, and for books no longer in the library. A book whose drive isn't connected keeps them, so it doesn't have to be measured again when the drive comes back.
- **Removed books** that have also been deleted from the disk lose their saved place. A removed book that is still on the disk, or on a drive that isn't connected, is remembered, so it isn't imported again.
- **Daily statistics** older than a year are folded into one summary. Total listening time, average speed and listening streaks stay exactly the same; the charts only show the last 26 weeks anyway. Each book's listening time and finished books are kept, because **Most listened** and **Books finished** use them.

To move to a new computer, copy the whole folder. Books that end up somewhere else are found again automatically (see below).

Data from earlier versions, which kept everything in `preferences.json`, is moved into `library.json` the first time the new version starts.

### Moved books

The app remembers each book by its files' names and sizes as well as its location, so moving books doesn't lose your progress, statistics or finished books:

- If a drive gets a different letter on Windows (for example a USB drive that was `E:` and is now `F:`), books and the library folder are found on the new letter.
- If you choose a new library folder after moving the old one, books are matched up by where they were inside it.
- If a book shows **Not found** and is then found by the library folder or **Add books** somewhere else, it is reconnected instead of being added a second time. This works even if the book's folder was renamed or a part was added.

### Opening files and running once

Only one copy of the app runs at a time. Starting it again, or opening an audiobook with it, brings the open window to the front instead. A file or folder passed on the command line (`"Audiobook Player.exe" "D:\Books\Dune.m4b"`) opens in the player, which is what Windows' **Open with** and file associations use.

### Media keys and system controls

The book you're listening to is shown in the system's media controls with its title, author and cover, and these all work while the app is in the background:

| Button | Action |
|---|---|
| Play, pause, play/pause | Play or pause |
| Next, skip forward | Forward 30 seconds |
| Previous, skip back | Back 30 seconds |
| Dragging the position (macOS) | Moves to that point in the book |

Next and previous skip 30 seconds rather than jumping to the next chapter, as most audiobook players do, so a headset's double press doesn't lose your place. On macOS, Control Center shows 30-second skip buttons and a position bar for the whole book. Windows shows the title, author and cover, and play/pause, previous and next buttons.

### Keyboard shortcuts

| Key | Action |
|---|---|
| Space | Play / pause |
| ← / → | Back / forward 30 seconds |
| ↑ / ↓ | Volume up / down |
| ⌘F / Ctrl+F | Search the library |

## Running from source

You need [Go 1.27 or newer](https://go.dev/dl/) and a C compiler, because Fyne uses OpenGL through cgo. With an older Go (1.21 or later) installed, `go` downloads 1.27 for you the first time you build.

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
go install fyne.io/tools/cmd/fyne@v1.7.3
fyne package -os darwin -release     # run on a Mac
fyne package -os windows -release    # run on Windows
```

Each package must be built on its own platform. To build for Windows from a Mac, use [fyne-cross](https://github.com/fyne-io/fyne-cross), which requires Docker.

### Windows installer

`packaging/windows/AudiobookPlayer.iss` is an [Inno Setup](https://jrsoftware.org/isinfo.php) script that turns `Audiobook Player.exe` into `dist\AudiobookPlayerSetup-<version>.exe`. The installer:

- installs for the current user without asking for administrator rights (or for everyone, if you choose),
- adds a Start menu entry and, if you tick it, a desktop shortcut,
- adds the app to **Open with** for `.m4b`, `.m4a`, `.mp3`, `.flac`, `.ogg`, `.oga` and `.wav` files, and to **Settings → Apps → Default apps**. Windows doesn't let installers make themselves the default. The first time you open a `.m4b` file, which usually has no other app, Windows suggests Audiobook Player,
- closes the app if it's running when you update, and
- leaves your library and settings behind when you uninstall.

On Windows, after `fyne package -os windows -release`:

```powershell
& "C:\Program Files (x86)\Inno Setup 6\ISCC.exe" /DAppVersion=1.0.0 packaging\windows\AudiobookPlayer.iss
```

### Automatic builds

`.github/workflows/build.yml` runs on GitHub Actions for every push and pull request:

1. **Tests** run on Windows and macOS (`go vet` and `go test`, with the race detector on macOS). This includes a test that registers with Windows' media controls for real. It skips itself on a Windows without them.
2. **Windows installer**: for pushes, the app is packaged and the installer built. You can download it from the run's **Artifacts**.
3. **Releases**: pushing a tag such as `v1.2.0` builds the installer with that version and attaches it to a new GitHub release:

```sh
git tag v1.2.0
git push origin v1.2.0
```

### Code signing

Without a signature, Windows SmartScreen warns that the installer is from an "unrecognised app", and users have to click **More info → Run anyway**. The workflow signs the app and the installer with [Azure Artifact Signing](https://learn.microsoft.com/azure/artifact-signing/) (formerly Trusted Signing) when it's set up, and skips signing when it isn't:

1. Create an Artifact Signing account, complete identity validation and create a certificate profile.
2. Create an app registration in Microsoft Entra ID with a client secret, and give it the **Artifact Signing Certificate Profile Signer** role on the account.
3. In the GitHub repository, under **Settings → Secrets and variables → Actions**, add:
   - secrets `AZURE_TENANT_ID`, `AZURE_CLIENT_ID` and `AZURE_CLIENT_SECRET`,
   - variables `SIGNING_ENDPOINT` (for example `https://weu.codesigning.azure.net/`), `SIGNING_ACCOUNT` and `SIGNING_PROFILE`.

A signature shows your verified name as the publisher straight away. SmartScreen's warning itself goes away once enough people have downloaded the signed app; no certificate skips that step any more.

## Project layout

```
main.go          starts the app and embeds the icon
docs/
  screenshots/   the pictures in this README
FyneApp.toml     app name, ID and version used by `fyne package`
audio/           playback: Player, decoders for each format, pitch-preserving speed changer
library/         books on disk: Book, folder scanning and sorting, tags, chapters, cover art, display names
stats/           listening statistics: recording, totals, streaks
store/           crash-safe saving, daily backups and recovery
instance/        making sure only one copy of the app runs
media/           media keys and the system's media controls (Windows, macOS)
titlebar/        a light or dark Windows title bar to match the app
packaging/
  windows/       the Inno Setup installer script and the Windows icon
.github/
  workflows/     tests on Windows and macOS, the Windows installer and releases
ui/              the window: creates the parts below and connects them
  app.go           start-up, loading and saving, opening files from the command line
  window.go        the tabs, drag and drop, keyboard shortcuts, the timers
  nowplaying/      the Now Playing tab: playback, parts and chapters, sleep timer, media keys, Open
  shelf/           the Library tab: cards, collections, search, sort and filter, adding and removing, moved books
  autoimport/      the library folder: checking it for new books, its section in Settings
  statistics/      the Statistics tab and its charts
  settings/        the Settings tab: Light / Dark and the Windows title bar
  userdata/        library.json: positions, part lengths, statistics, removed books, older versions
  kit/             shared look: colours, icons, time formats, headings, menus, file pickers
  internal/
    testbooks/     sample books for the tests
```

`audio` and `library` don't depend on the UI, so they can be tested without opening a window.

Each part of `ui` keeps its widgets and state private, and the parts don't use each other's internals. `ui` gives each part what it needs: the saved data (`userdata`), or a small named function such as the library's `Open` (play this book) and `Show` (show the Library tab). So changing how one tab works can only affect another tab through those functions. Only `ui` itself needs a C compiler, so the other parts can be checked for Windows from a Mac, for example `GOOS=windows CGO_ENABLED=0 go vet ./ui/shelf/ ./ui/nowplaying/`.

## Tests

```sh
go test ./...
```

Small sample audio files for the decoder tests are in `audio/testdata` (see the README there for their sources).

### Screenshots

The pictures in this README are drawn by the app itself, from a demo library of public-domain books with made-up cover art and listening history. After changing how the app looks, update them with:

```sh
go test -tags screenshots -run TestScreenshots ./ui/
```

This writes `docs/screenshots/*.png`. The generator is in `ui/screenshots_test.go`, and normal test runs skip it.
