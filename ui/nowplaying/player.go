package nowplaying

import (
    "image/color"
    "sync/atomic"
    "time"

    "fyne.io/fyne/v2"
    "fyne.io/fyne/v2/canvas"
    "fyne.io/fyne/v2/container"
    "fyne.io/fyne/v2/layout"
    "fyne.io/fyne/v2/theme"
    "fyne.io/fyne/v2/widget"

    "github.com/izzzicos/audiobook-player/audio"
    "github.com/izzzicos/audiobook-player/library"
    "github.com/izzzicos/audiobook-player/ui/about"
    "github.com/izzzicos/audiobook-player/ui/details"
    "github.com/izzzicos/audiobook-player/ui/kit"
    "github.com/izzzicos/audiobook-player/ui/offline"
    "github.com/izzzicos/audiobook-player/ui/userdata"
)

const waitingText = "Waiting for the drive or server…"

const (
    preferenceVolume     = "volume"
    preferenceSpeed      = "speed"
    preferenceWasPlaying = "wasPlaying"
)

type Player struct {
    OnBookOpened func(key string)
    EditDetails  func(key string, original details.Original)
    FolderBookOf func(file string) string
    ShowAbout    func(key string, book about.Book)
    OnPaused       func()
    OnSpeedChanged func(key string)
    Offline      *offline.Store

    window fyne.Window
    prefs  fyne.Preferences
    data   *userdata.Data
    picker *kit.Picker
    async  func(func())

    content          fyne.CanvasObject
    title            *widget.RichText
    subtitle         *widget.RichText
    progress         *widget.Slider
    current          *widget.RichText
    remaining        *widget.RichText
    cover            *canvas.Image
    coverPlaceholder fyne.CanvasObject
    volume           *widget.Slider
    speedButton      *widget.Button
    backdrop         *canvas.LinearGradient
    playButton       *roundButton
    previousButton   *roundButton
    nextButton       *roundButton
    partsButton      *widget.Button
    editButton       *widget.Button
    aboutButton      *widget.Button
    sleepButton      *widget.Button

    audio          *audio.Player
    book           *library.Book
    playing        bool
    speed          float64
    speedChoice    string
    backdropTint   *color.NRGBA
    coverName      string
    chapterIndex   int
    sleep          sleepState
    metadata       library.Metadata
    tags           library.Metadata
    foundCover     *library.Cover
    chapters       []library.Chapter
    media          mediaControls
    mediaStatus    publishedStatus
    seeking        atomic.Bool
    waiting        bool

    pausedAt           time.Time
    clock              func() time.Time

    generation         int
    opening            bool
    playAfterOpen      bool
    prefetched         *openedAudio
    prefetching        string
    prefetchGeneration int
}

func New(window fyne.Window, prefs fyne.Preferences, data *userdata.Data, picker *kit.Picker, async func(func())) *Player {
    p := &Player{
        window: window,
        prefs:  prefs,
        data:   data,
        picker: picker,
        async:  async,
        speed:  1.0,
        sleep:  sleepState{lastPosition: noSleepPosition},
        clock:  time.Now,
    }
    p.build()

    return p
}

func (p *Player) build() {
    titleStyle := widget.RichTextStyleHeading
    titleStyle.Alignment = fyne.TextAlignCenter
    p.title = newRichText("Open or drop an audiobook", titleStyle)

    subtitleStyle := widget.RichTextStyleInline
    subtitleStyle.Alignment = fyne.TextAlignCenter
    subtitleStyle.ColorName = theme.ColorNamePlaceHolder
    p.subtitle = newRichText("", subtitleStyle)
    p.subtitle.Hide()

    p.current = newRichText(kit.FormatTime(0), newCaptionStyle(fyne.TextAlignLeading))
    p.current.Truncation = fyne.TextTruncateOff
    p.remaining = newRichText("−"+kit.FormatTime(0), newCaptionStyle(fyne.TextAlignTrailing))
    p.remaining.Truncation = fyne.TextTruncateOff

    p.progress = widget.NewSlider(0, 1)
    p.progress.OnChanged = p.previewSeek
    p.progress.OnChangeEnded = p.finishSeek

    p.volume = widget.NewSlider(0, 1)
    p.volume.Step = 0.01
    p.volume.SetValue(0.7)
    p.volume.OnChanged = func(value float64) {
        if p.audio != nil {
            p.audio.SetVolume(value)
        }
    }
    p.volume.OnChangeEnded = func(float64) {
        p.releaseFocus()
    }

    p.speedButton = widget.NewButton("", p.showSpeedMenu)
    p.selectSpeed("1.0x")

    p.backdrop = canvas.NewVerticalGradient(color.Transparent, color.Transparent)
    kit.FollowTheme(p.applyBackdrop)

    p.content = container.New(
        layout.NewCustomPaddedLayout(4, 12, 16, 16),
        container.NewBorder(
            p.buildTopBar(),
            p.buildNowPlaying(),
            nil,
            nil,
            container.New(layout.NewCustomPaddedLayout(8, 20, 24, 24), p.buildCover()),
        ),
    )
}

func (p *Player) Content() fyne.CanvasObject {
    return p.content
}

func (p *Player) Backdrop() fyne.CanvasObject {
    return p.backdrop
}

func (p *Player) ShowBackdrop(visible bool) {
    if visible {
        p.backdrop.Show()
    } else {
        p.backdrop.Hide()
    }
}

func (p *Player) Book() *library.Book {
    return p.book
}

func (p *Player) Playing() bool {
    return p.playing
}

func (p *Player) Speed() float64 {
    return p.speed
}

func (p *Player) Play() {
    if p.audio != nil && !p.playing {
        p.TogglePlay()
    }
}

func (p *Player) Tick(now time.Time) {
    p.refreshProgress()
    p.tickSleepTimer(now)
    p.showWaiting()
    p.prefetchNextPart()
}

func (p *Player) showWaiting() {
    if p.opening {
        return
    }

    waiting := p.audio != nil && p.playing && p.audio.Waiting()
    if waiting == p.waiting {
        return
    }

    p.waiting = waiting
    if waiting {
        p.setSubtitle(waitingText)
    } else if p.book != nil {
        p.setSubtitle(p.subtitleText())
    }
}

func (p *Player) HandleKey(event *fyne.KeyEvent) {
    switch event.Name {
    case fyne.KeySpace:
        p.TogglePlay()
    case fyne.KeyLeft:
        p.Skip(-skipSeconds)
    case fyne.KeyRight:
        p.Skip(skipSeconds)
    case fyne.KeyUp:
        p.changeVolume(0.05)
    case fyne.KeyDown:
        p.changeVolume(-0.05)
    }
}

func (p *Player) releaseFocus() {
    p.window.Canvas().Unfocus()
}

func (p *Player) LoadSettings() {
    p.volume.SetValue(p.prefs.FloatWithFallback(preferenceVolume, 0.7))
    p.selectSpeed(p.prefs.StringWithFallback(preferenceSpeed, "1.0x"))
}

func (p *Player) SaveSettings() {
    p.prefs.SetFloat(preferenceVolume, p.volume.Value)

    if p.book != nil && p.audio != nil {
        p.prefs.SetBool(preferenceWasPlaying, p.playing)
        p.SavePosition()
    }
}

func (p *Player) SavePosition() {
    if p.book == nil || p.audio == nil {
        return
    }

    p.data.Remember(p.book, p.positionSeconds())
}

func (p *Player) markFinished() {
    if p.book != nil {
        p.data.Finish(p.book.Key())
    }
}
