package ui

import (
    "cmp"
    "fmt"
    "os"
    "path/filepath"
    "strings"

    "fyne.io/fyne/v2"
    "fyne.io/fyne/v2/app"
    "fyne.io/fyne/v2/container"
    "fyne.io/fyne/v2/dialog"

    "github.com/izzzicos/audiobook-player/audio"
    "github.com/izzzicos/audiobook-player/instance"
    "github.com/izzzicos/audiobook-player/library"
    "github.com/izzzicos/audiobook-player/ui/about"
    "github.com/izzzicos/audiobook-player/ui/autoimport"
    "github.com/izzzicos/audiobook-player/ui/details"
    "github.com/izzzicos/audiobook-player/ui/kit"
    "github.com/izzzicos/audiobook-player/ui/nowplaying"
    "github.com/izzzicos/audiobook-player/ui/offline"
    "github.com/izzzicos/audiobook-player/ui/remote"
    "github.com/izzzicos/audiobook-player/ui/settings"
    "github.com/izzzicos/audiobook-player/ui/shelf"
    "github.com/izzzicos/audiobook-player/ui/statistics"
    "github.com/izzzicos/audiobook-player/ui/syncing"
    "github.com/izzzicos/audiobook-player/ui/userdata"
    "github.com/izzzicos/audiobook-player/vfs"
)

const AppID = "com.izzzicos.audiobookplayer"

type UI struct {
    App    fyne.App
    Window fyne.Window

    data    *userdata.Data
    async   func(func())
    secrets remote.Secrets

    picker     *kit.Picker
    editor     *details.Editor
    about      *about.About
    player     *nowplaying.Player
    shelf      *shelf.Shelf
    importer   *autoimport.Importer
    syncer     *syncing.Syncer
    offline    *offline.Store
    remote     *remote.Connection
    statistics *statistics.View
    settings   *settings.Settings

    content     fyne.CanvasObject
    tabs        *container.AppTabs
    playerTab   *container.TabItem
    libraryTab  *container.TabItem
    statsTab    *container.TabItem
    settingsTab *container.TabItem
}

func NewUI(icon fyne.Resource) *UI {
    a := app.NewWithID(AppID)
    if icon != nil {
        a.SetIcon(icon)
    }

    w := a.NewWindow("Audiobook Player")
    w.Resize(fyne.NewSize(620, 780))
    a.Settings().SetTheme(settings.SavedTheme(a.Preferences()))

    return &UI{
        App:    a,
        Window: w,
        data:   userdata.New(userdata.FileIn(userdata.Folder(a.Storage().RootURI().Path()))),
        async: func(work func()) {
            go work()
        },
        secrets: remote.Keychain{},
    }
}

func (ui *UI) Run(args []string) {
    server, forwarded, err := instance.Claim(ui.instanceSocket(), args)
    if forwarded {
        return
    }
    if err != nil {
        fyne.LogError("Another copy of the app may be able to start at the same time", err)
    }

    ui.setup()
    ui.openArguments(args)

    if server != nil {
        defer server.Close()
        go server.Serve(func(args []string) {
            fyne.Do(func() {
                ui.Window.Show()
                ui.Window.RequestFocus()
                ui.openArguments(args)
            })
        })
    }

    ui.Window.Show()
    ui.settings.MatchTitleBar()
    ui.App.Run()
    ui.player.StopMediaControls()
}

func (ui *UI) instanceSocket() string {
    socket := filepath.Join(filepath.Dir(ui.data.File()), "instance.sock")
    if len(socket) > 100 {
        socket = filepath.Join(os.TempDir(), AppID+".sock")
    }

    return socket
}

func (ui *UI) build() {
    prefs := ui.App.Preferences()
    ui.picker = &kit.Picker{}

    ui.data.OnSaveError = func(err error) {
        dialog.ShowError(fmt.Errorf("your library and listening progress couldn't be saved: %w", err), ui.Window)
    }

    ui.editor = details.New(ui.Window, ui.picker, ui.data, ui.async)
    ui.about = about.New(ui.Window, prefs, ui.data, ui.async)
    ui.player = nowplaying.New(ui.Window, prefs, ui.data, ui.picker, ui.async)
    ui.shelf = shelf.New(ui.Window, prefs, ui.data, ui.picker, ui.async)
    ui.importer = autoimport.New(ui.Window, prefs, ui.data, ui.shelf, ui.picker, ui.async)
    ui.statistics = statistics.New(ui.data, ui.shelf.Title)
    ui.syncer = syncing.New(ui.Window, prefs, ui.data, ui.picker, ui.async, ui.player)
    ui.offline = offline.New(ui.Window, prefs, ui.data, ui.async)
    if ui.secrets == nil {
        ui.secrets = remote.Memory{}
    }
    ui.remote = remote.New(ui.Window, prefs, ui.picker, ui.secrets, filepath.Dir(ui.data.File()))
    ui.settings = settings.New(ui.App, ui.Window, ui.player.Section(), ui.remote.Section(), ui.importer.Section(), ui.offline.Section(), ui.syncer.Section(), ui.about.Section())

    ui.player.OnBookOpened = func(key string) {
        ui.shelf.Add([]string{key})
        ui.offline.BookOpened(key)
    }
    ui.player.Offline = ui.offline
    ui.shelf.Offline = ui.offline
    ui.offline.OnChanged = ui.shelf.DownloadChanged
    ui.remote.UseAsLibrary = ui.importer.SetFolder
    ui.remote.SyncThrough = ui.syncer.SetFolder
    ui.remote.OnConnected = ui.serverConnected
    ui.shelf.OnRestart = func(path string) {
        ui.player.Restart(path)
        ui.syncer.Write()
    }
    ui.offline.Title = ui.shelf.Title
    ui.shelf.Open = ui.playFromShelf
    ui.shelf.Show = ui.showLibrary
    ui.shelf.SpeedOf = ui.player.SpeedFor
    ui.player.OnSpeedChanged = ui.shelf.Reload
    ui.shelf.SyncProgress = ui.player.SavePosition
    ui.shelf.StartFolder = ui.player.LibraryFolder
    ui.importer.StartFolder = ui.player.LibraryFolder
    ui.syncer.StartFolder = ui.importer.Folder
    ui.syncer.Title = ui.shelf.Title
    ui.syncer.OnChanged = ui.shelf.Refresh
    ui.player.OnPaused = ui.syncer.Write

    ui.player.FolderBookOf = ui.shelf.FolderBookOf
    ui.player.EditDetails = ui.editor.Show
    ui.shelf.EditDetails = ui.editor.Show
    ui.player.ShowAbout = ui.about.Show
    ui.shelf.ShowAbout = ui.about.Show
    ui.about.OnLinked = func(string) {
        ui.shelf.Refresh()
    }
    ui.editor.OnSaved = func(key string) {
        ui.player.RefreshDetails(key)
        ui.shelf.Reload(key)
    }

    ui.content = ui.buildTabs()
}

func (ui *UI) setup() {
    ui.build()

    ui.Window.SetPadded(false)
    ui.Window.SetContent(ui.content)
    ui.App.Lifecycle().SetOnStopped(func() {
        ui.save()
        ui.remote.Close()
    })
    ui.App.Lifecycle().SetOnEnteredForeground(func() {
        ui.syncer.Pull(nil)
    })
    ui.App.Lifecycle().SetOnStarted(func() {
        kit.FollowTheme(ui.settings.MatchTitleBar)
    })
    ui.Window.Canvas().SetOnTypedKey(ui.player.HandleKey)
    ui.addShortcuts()
    ui.Window.SetOnDropped(ui.handleDrop)

    if err := audio.InitSpeaker(); err != nil {
        dialog.ShowError(err, ui.Window)
    }

    ui.player.StartMediaControls()
    ui.load()

    go ui.tick()
}

func (ui *UI) load() {
    ui.player.LoadSettings()

    if notice := ui.data.Load(ui.App.Preferences()); notice != "" {
        dialog.ShowInformation("Library restored", notice, ui.Window)
    }
    ui.remote.Load()
    ui.shelf.Load()
    ui.offline.Load()
    ui.tidy()
    ui.importer.Load()
    ui.syncer.Load()

    ui.resumeLastBook()
}

func (ui *UI) serverConnected() {
    ui.shelf.RecheckMissing()
    ui.importer.Scan()
    ui.syncer.Pull(nil)

    last := cmp.Or(ui.data.LastBook.Folder, ui.data.LastBook.File)
    if last != "" && vfs.IsRemote(last) && ui.player.Book() == nil && !ui.player.Opening() {
        ui.resumeLastBook()
    }
}

func (ui *UI) resumeLastBook() {
    ui.syncer.Pull(func(elsewhere *syncing.Change) {
        if elsewhere != nil && elsewhere.Record.Listened > ui.lastListened() {
            ui.data.LastBook = elsewhere.LastBook()
        }
        ui.resumeLocalBook()
    })
}

func (ui *UI) lastListened() int64 {
    key := ui.data.LastBook.Folder
    if key == "" {
        key = ui.data.LastBook.File
    }

    return ui.data.Positions[key].Listened
}

func (ui *UI) resumeLocalBook() {
    last := ui.data.LastBook
    lastKey := cmp.Or(last.Folder, last.File)
    if last.File == "" {
        ui.showLibrary()
        return
    }

    ui.async(func() {
        err := library.Reachable(last.File, library.ResponseTimeout)
        if ui.offline.Local(lastKey) != nil {
            err = nil
        }

        fyne.Do(func() {
            if err == nil && ui.player.Book() == nil {
                ui.player.Resume(last.Folder, last.File, 0)
            }
            if ui.player.Book() == nil && !ui.player.Opening() {
                ui.showLibrary()
            }
        })
    })
}

func (ui *UI) tidy() {
    plan := ui.data.PlanTidy()
    ui.async(func() {
        leftovers := plan.Find()
        fyne.Do(func() {
            ui.data.Forget(leftovers)
        })
    })
}

func (ui *UI) save() {
    ui.player.SaveSettings()
    ui.data.Flush()
    ui.syncer.Flush()
}

func (ui *UI) openArguments(args []string) {
    for _, arg := range args {
        if strings.HasPrefix(arg, "-") {
            continue
        }

        info, err := os.Stat(arg)
        switch {
        case err != nil:
            continue
        case info.IsDir():
            ui.player.OpenFolder(arg)
        case audio.IsSupported(arg):
            ui.player.OpenFile(arg)
        default:
            continue
        }

        ui.tabs.Select(ui.playerTab)
        return
    }
}
