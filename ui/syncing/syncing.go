package syncing

import (
    "crypto/rand"
    "encoding/hex"
    "encoding/json"
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "slices"
    "strings"
    "sync/atomic"
    "time"

    "fyne.io/fyne/v2"
    "fyne.io/fyne/v2/canvas"
    "fyne.io/fyne/v2/container"
    "fyne.io/fyne/v2/dialog"
    "fyne.io/fyne/v2/theme"
    "fyne.io/fyne/v2/widget"

    "github.com/ncruces/zenity"

    "github.com/izzzicos/audiobook-player/library"
    "github.com/izzzicos/audiobook-player/ui/kit"
    "github.com/izzzicos/audiobook-player/ui/userdata"
    "github.com/izzzicos/audiobook-player/vfs"
)

const (
    preferenceFolder   = "syncFolder"
    preferenceDeviceID = "syncDeviceID"
    syncFolderName     = ".audiobook-player-sync"
    writeEvery         = time.Minute
    pullEvery          = time.Minute
    quitWriteTimeout   = 3 * time.Second
)

var reachable = library.Reachable

type Player interface {
    Book() *library.Book
    Playing() bool
    MoveTo(part string, seconds float64) bool
}

type Syncer struct {
    StartFolder func() string
    Title       func(path string) (string, bool)
    OnChanged   func()

    window fyne.Window
    prefs  fyne.Preferences
    data   *userdata.Data
    picker *kit.Picker
    async  func(func())
    player Player

    folder    string
    id        string
    name      string
    lastWrite time.Time
    lastPull  time.Time
    writing   atomic.Bool
    pulling   atomic.Bool
    dismissed map[string]int64
    prompting bool

    section       fyne.CanvasObject
    folderLabel   *widget.Label
    folderActions *fyne.Container
    status        *widget.Label
}

func New(window fyne.Window, prefs fyne.Preferences, data *userdata.Data, picker *kit.Picker, async func(func()), player Player) *Syncer {
    s := &Syncer{
        window:    window,
        prefs:     prefs,
        data:      data,
        picker:    picker,
        async:     async,
        player:    player,
        dismissed: map[string]int64{},
        name:      deviceName(),
    }
    s.build()

    return s
}

func deviceName() string {
    name, err := os.Hostname()
    if err != nil || name == "" {
        return "Another computer"
    }

    return strings.TrimSuffix(strings.TrimSuffix(name, ".local"), ".lan")
}

func (s *Syncer) deviceID() string {
    if s.id != "" {
        return s.id
    }

    s.id = s.prefs.String(preferenceDeviceID)
    if s.id == "" {
        random := make([]byte, 8)
        if _, err := rand.Read(random); err != nil {
            random = []byte(fmt.Sprint(time.Now().UnixNano()))
        }
        s.id = hex.EncodeToString(random)
        s.prefs.SetString(preferenceDeviceID, s.id)
    }

    return s.id
}

func (s *Syncer) Section() fyne.CanvasObject {
    return s.section
}

func (s *Syncer) Folder() string {
    return s.folder
}

func (s *Syncer) Enabled() bool {
    return s.folder != ""
}

func (s *Syncer) build() {
    s.folderLabel = widget.NewLabel("")
    s.folderLabel.Truncation = fyne.TextTruncateEllipsis

    background := canvas.NewRectangle(theme.Color(theme.ColorNameInputBackground))
    background.CornerRadius = theme.InputRadiusSize()
    kit.FollowTheme(func() {
        background.FillColor = theme.Color(theme.ColorNameInputBackground)
        background.Refresh()
    })
    folderRow := container.NewStack(
        background,
        container.NewBorder(nil, nil, container.NewPadded(widget.NewIcon(theme.FolderIcon())), nil, s.folderLabel),
    )

    choose := widget.NewButtonWithIcon("Choose folder…", theme.FolderOpenIcon(), s.chooseFolder)
    syncNow := widget.NewButtonWithIcon("Sync now", theme.ViewRefreshIcon(), func() {
        s.Write()
        s.Pull(nil)
    })
    stop := widget.NewButtonWithIcon("Stop syncing", theme.ContentClearIcon(), func() {
        s.SetFolder("")
    })
    stop.Importance = widget.LowImportance
    s.folderActions = container.NewHBox(syncNow, stop)

    s.status = widget.NewLabel("")
    s.status.SizeName = theme.SizeNameCaptionText
    s.status.Importance = widget.LowImportance
    s.status.Wrapping = fyne.TextWrapWord

    s.updateFolder()

    s.section = kit.Section(
        "Sync between computers",
        "Choose a folder that all your computers can reach, such as a folder on your NAS, or one kept in step by "+
            "Synology Drive, iCloud Drive, Dropbox or OneDrive. Each computer saves its place in every book there, "+
            "so you can carry on where you stopped on another computer. The books themselves need to be on each computer or on the NAS.",
        folderRow,
        container.NewBorder(nil, nil, choose, nil, s.folderActions),
        s.status,
    )
}

func (s *Syncer) updateFolder() {
    if s.folder == "" {
        s.folderLabel.SetText("Not syncing")
        s.folderLabel.Importance = widget.LowImportance
        s.folderActions.Hide()
        s.status.SetText("Your place in each book is only kept on this computer.")
    } else {
        s.folderLabel.SetText(vfs.Display(s.folder))
        s.folderLabel.Importance = widget.MediumImportance
        s.folderActions.Show()
    }
    s.folderLabel.Refresh()
}

func (s *Syncer) chooseFolder() {
    start := s.folder
    if start == "" && s.StartFolder != nil {
        start = s.StartFolder()
    }
    if start == "" {
        start = kit.HomeFolder()
    }

    kit.Browse(s.picker, s.window,
        func() (string, error) {
            return zenity.SelectFile(
                zenity.Title("Choose a folder to sync through"),
                zenity.Filename(kit.WithTrailingSeparator(start)),
                zenity.Directory(),
            )
        },
        s.SetFolder,
        func() {
            kit.ShowFolderDialog(s.window, start, s.SetFolder)
        },
    )
}

func (s *Syncer) Load() {
    s.folder = s.prefs.String(preferenceFolder)
    s.updateFolder()
}

func (s *Syncer) SetFolder(folder string) {
    if folder != "" {
        folder = library.Normalize(folder)
    }

    s.folder = folder
    s.prefs.SetString(preferenceFolder, folder)
    s.updateFolder()

    if folder != "" {
        s.Write()
        s.Pull(nil)
    }
}

func (s *Syncer) syncDirectory(folder string) string {
    return filepath.Join(folder, syncFolderName)
}

func (s *Syncer) snapshot() Device {
    device := Device{Version: deviceVersion, ID: s.deviceID(), Name: s.name, Written: time.Now().Unix()}

    last := s.data.LastBook.Folder
    if last == "" {
        last = s.data.LastBook.File
    }

    for _, book := range s.data.Library {
        position, found := s.data.Positions[book.Path]
        if !found {
            continue
        }

        title := ""
        if s.Title != nil {
            title, _ = s.Title(book.Path)
        }
        record, ok := recordFor(book, title, position, s.data.Durations)
        if !ok {
            continue
        }
        record.Last = book.Path == last
        device.Books = append(device.Books, record)
    }

    return device
}

func (s *Syncer) Write() {
    folder := s.folder
    if folder == "" || !s.writing.CompareAndSwap(false, true) {
        return
    }

    device := s.snapshot()
    s.lastWrite = time.Now()

    s.async(func() {
        err := s.write(folder, device)

        fyne.Do(func() {
            s.writing.Store(false)
            if err != nil && folder == s.folder {
                s.showProblem(err)
            }
        })
    })
}

func (s *Syncer) write(folder string, device Device) error {
    if err := reachable(folder, library.ResponseTimeout); err != nil {
        return err
    }

    data, err := json.Marshal(device)
    if err != nil {
        return err
    }

    return vfs.WriteAtomic(filepath.Join(s.syncDirectory(folder), device.ID+".json"), data)
}

func (s *Syncer) Flush() {
    folder := s.folder
    if folder == "" {
        return
    }

    device := s.snapshot()
    done := make(chan error, 1)
    go func() {
        done <- s.write(folder, device)
    }()

    select {
    case err := <-done:
        if err != nil {
            fyne.LogError("Saving your place to the sync folder failed", err)
        }
    case <-time.After(quitWriteTimeout):
        fyne.LogError("Saving your place to the sync folder took too long", library.ErrNotResponding)
    }
}

func (s *Syncer) Pull(done func(last *Change)) {
    folder := s.folder
    if folder == "" || !s.pulling.CompareAndSwap(false, true) {
        if done != nil {
            done(nil)
        }
        return
    }

    id := s.deviceID()
    s.lastPull = time.Now()

    s.async(func() {
        devices, err := s.read(folder, id)

        fyne.Do(func() {
            s.pulling.Store(false)

            var last *Change
            if folder == s.folder {
                if err != nil {
                    s.showProblem(err)
                } else {
                    last = s.apply(devices)
                    s.showSynced(devices)
                }
            }

            if done != nil {
                done(last)
            }
        })
    })
}

func (s *Syncer) read(folder, id string) ([]Device, error) {
    if err := reachable(folder, library.ResponseTimeout); err != nil {
        return nil, err
    }

    entries, err := vfs.ReadDir(s.syncDirectory(folder))
    if errors.Is(err, os.ErrNotExist) {
        return nil, nil
    }
    if err != nil {
        return nil, err
    }

    var devices []Device
    for _, entry := range entries {
        name := entry.Name()
        if entry.IsDir() || filepath.Ext(name) != ".json" || strings.TrimSuffix(name, ".json") == id {
            continue
        }

        data, err := vfs.ReadFile(filepath.Join(s.syncDirectory(folder), name))
        if err != nil {
            continue
        }

        var device Device
        if err := json.Unmarshal(data, &device); err != nil || device.Version != deviceVersion || device.ID == id {
            continue
        }
        devices = append(devices, device)
    }

    return devices, nil
}

func (s *Syncer) apply(devices []Device) *Change {
    changes, last := newerChanges(s.data.Library, s.data.Positions, s.data.Durations, devices)

    current := ""
    if book := s.player.Book(); book != nil {
        current = book.Key()
    }

    changed := false
    for _, change := range changes {
        if change.Book.Path == current {
            s.offer(change)
            continue
        }

        s.data.Positions[change.Book.Path] = change.Position
        changed = true
    }

    if changed {
        s.data.MarkDirty()
        if s.OnChanged != nil {
            s.OnChanged()
        }
    }

    return last
}

func (s *Syncer) offer(change Change) {
    path := change.Book.Path
    if s.player.Playing() || s.prompting || s.dismissed[path] >= change.Record.Listened {
        return
    }

    title := change.Record.Title
    if s.Title != nil {
        if local, found := s.Title(path); found {
            title = local
        }
    }

    s.prompting = true
    confirm := dialog.NewConfirm(
        "Continue from "+placeText(change.Record)+"?",
        fmt.Sprintf("You listened to “%s” on %s %s. Jump to where you stopped there?", title, change.Device, ago(change.Record.Listened, time.Now())),
        func(jump bool) {
            s.prompting = false
            if jump {
                s.player.MoveTo(change.Position.Part, change.Position.Position)
                return
            }
            s.dismissed[path] = change.Record.Listened
        },
        s.window,
    )
    confirm.SetConfirmText("Jump there")
    confirm.SetDismissText("Stay here")
    confirm.Show()
}

func placeText(record Record) string {
    if record.Elapsed > 0 {
        return kit.FormatTime(record.Elapsed)
    }

    return fmt.Sprintf("part %d at %s", record.PartIndex+1, kit.FormatTime(record.Position))
}

func ago(listened int64, now time.Time) string {
    elapsed := now.Sub(time.Unix(listened, 0))
    switch {
    case elapsed < time.Minute:
        return "just now"
    case elapsed < time.Hour:
        return kit.CountPhrase(int(elapsed/time.Minute), "minute ago", "minutes ago", "")
    case elapsed < 48*time.Hour:
        return kit.CountPhrase(int(elapsed/time.Hour), "hour ago", "hours ago", "")
    default:
        return "on " + time.Unix(listened, 0).Format("2 January")
    }
}

func (s *Syncer) showSynced(devices []Device) {
    checked := time.Now().Format("15:04")
    if len(devices) == 0 {
        s.status.SetText(fmt.Sprintf("Synced at %s. No other computer has synced through this folder yet.", checked))
        return
    }

    var names []string
    for _, device := range devices {
        names = append(names, device.Name)
    }
    slices.Sort(names)
    s.status.SetText(fmt.Sprintf("Synced at %s with %s.", checked, strings.Join(names, ", ")))
}

func (s *Syncer) showProblem(err error) {
    checked := time.Now().Format("15:04")
    switch {
    case errors.Is(err, library.ErrNotResponding):
        s.status.SetText(fmt.Sprintf("The sync folder isn't responding (at %s). Your place is kept here and synced once it's back.", checked))
    case errors.Is(err, os.ErrNotExist):
        s.status.SetText(fmt.Sprintf("The sync folder isn't available (at %s). Your place is kept here and synced once it's back.", checked))
    default:
        s.status.SetText(fmt.Sprintf("Syncing failed at %s: %v", checked, err))
    }
}

func (s *Syncer) Tick(now time.Time) {
    if s.folder == "" {
        return
    }

    if s.player.Playing() && now.Sub(s.lastWrite) >= writeEvery {
        s.Write()
    }
    if now.Sub(s.lastPull) >= pullEvery {
        s.Pull(nil)
    }
}

func (s *Syncer) Status() string {
    return s.status.Text
}
