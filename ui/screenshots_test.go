//go:build screenshots

package ui

import (
    "bytes"
    "encoding/binary"
    "fmt"
    "image"
    "image/color"
    "image/png"
    "math"
    "os"
    "path/filepath"
    "testing"
    "time"

    "fyne.io/fyne/v2"
    "fyne.io/fyne/v2/container"
    "fyne.io/fyne/v2/test"
    "fyne.io/fyne/v2/theme"

    "github.com/izzzicos/audiobook-player/stats"
    "github.com/izzzicos/audiobook-player/ui/kit"
    "github.com/izzzicos/audiobook-player/ui/userdata"
)

const (
    screenshotFolder = "../docs/screenshots"
    screenshotScale  = 2
    sampleRecording  = "../audio/testdata/speech-16bit-mono.flac"
    playingBook      = "Frankenstein"
)

type demoBook struct {
    title    string
    author   string
    parts    int
    hours    float64
    progress float64
    hoursAgo int
    sky      color.NRGBA
    horizon  color.NRGBA
    sun      color.NRGBA
    ink      color.NRGBA
}

func rgb(value uint32) color.NRGBA {
    return color.NRGBA{R: uint8(value >> 16), G: uint8(value >> 8), B: uint8(value), A: 255}
}

var demoBooks = []demoBook{
    {"Frankenstein", "Mary Shelley", 24, 8.6, 0.43, 1, rgb(0x1c2b3a), rgb(0x7fa38a), rgb(0xe9f0d8), rgb(0x0e1a14)},
    {"Pride and Prejudice", "Jane Austen", 61, 11.6, 0.78, 26, rgb(0x6b2c3e), rgb(0xf0b49a), rgb(0xfff1df), rgb(0x3a1622)},
    {"Moby-Dick", "Herman Melville", 135, 24.8, 0.12, 50, rgb(0x0f2a4a), rgb(0x5f9cc4), rgb(0xf4f7f2), rgb(0x081a2e)},
    {"The Time Machine", "H. G. Wells", 12, 3.4, 1, 74, rgb(0x2d1f4f), rgb(0xe8946a), rgb(0xffe2b8), rgb(0x1a1030)},
    {"Treasure Island", "Robert Louis Stevenson", 34, 7.1, 1, 140, rgb(0x0d4f55), rgb(0xf2c45a), rgb(0xfff4cf), rgb(0x063236)},
    {"Dracula", "Bram Stoker", 27, 15.5, 0, 0, rgb(0x1f0a10), rgb(0x8f2a33), rgb(0xf3e3e0), rgb(0x0f0407)},
    {"Little Women", "Louisa May Alcott", 47, 18.9, 0, 0, rgb(0x55705a), rgb(0xeadbbd), rgb(0xfffaf0), rgb(0x2f4234)},
    {"The Adventures of Sherlock Holmes", "Arthur Conan Doyle", 12, 12.2, 1, 300, rgb(0x2e2119), rgb(0xc2895a), rgb(0xf7e6cf), rgb(0x1a120d)},
}

func mix(a, b color.NRGBA, amount float64) color.NRGBA {
    blend := func(x, y uint8) uint8 {
        return uint8(float64(x)*(1-amount) + float64(y)*amount + 0.5)
    }
    return color.NRGBA{R: blend(a.R, b.R), G: blend(a.G, b.G), B: blend(a.B, b.B), A: 255}
}

func coverColour(book demoBook, x, y, size float64) color.NRGBA {
    shade := mix(book.sky, book.horizon, math.Pow(y/size, 1.4))

    spread := math.Mod(float64(len(book.title))*0.37, 1)
    sunX, sunY, sunRadius := size*(0.3+0.4*spread), size*(0.33+0.1*spread), size*(0.13+0.05*spread)
    if math.Hypot(x-sunX, y-sunY) < sunRadius {
        shade = book.sun
    }

    for layer := range 3 {
        depth := float64(layer)
        ground := size*(0.6+0.11*depth) + size*0.035*math.Sin(x/size*(5+depth*2)+depth*1.7+float64(len(book.title)))
        if y > ground {
            shade = mix(book.horizon, book.ink, 0.45+0.25*depth)
        }
    }

    return shade
}

func coverArt(book demoBook) []byte {
    const size, samples = 480, 2

    picture := image.NewNRGBA(image.Rect(0, 0, size, size))
    for y := range size {
        for x := range size {
            var r, g, b float64
            for sy := range samples {
                for sx := range samples {
                    c := coverColour(book, float64(x)+(float64(sx)+0.5)/samples, float64(y)+(float64(sy)+0.5)/samples, size)
                    r, g, b = r+float64(c.R), g+float64(c.G), b+float64(c.B)
                }
            }
            count := float64(samples * samples)
            picture.SetNRGBA(x, y, color.NRGBA{R: uint8(r / count), G: uint8(g / count), B: uint8(b / count), A: 255})
        }
    }

    var encoded bytes.Buffer
    png.Encode(&encoded, picture)
    return encoded.Bytes()
}

func taggedRecording(t *testing.T, destination string, tags map[string]string) {
    t.Helper()

    sample, err := os.ReadFile(sampleRecording)
    if err != nil {
        t.Fatal(err)
    }

    out := []byte("fLaC")
    position := 4
    for {
        header := sample[position]
        length := int(sample[position+1])<<16 | int(sample[position+2])<<8 | int(sample[position+3])
        if header&0x7f != 4 {
            block := append([]byte{header & 0x7f}, sample[position+1:position+4+length]...)
            out = append(out, block...)
        }
        position += 4 + length
        if header&0x80 != 0 {
            break
        }
    }

    var comment bytes.Buffer
    vendor := "audiobook-player"
    binary.Write(&comment, binary.LittleEndian, uint32(len(vendor)))
    comment.WriteString(vendor)
    binary.Write(&comment, binary.LittleEndian, uint32(len(tags)))
    for key, value := range tags {
        field := key + "=" + value
        binary.Write(&comment, binary.LittleEndian, uint32(len(field)))
        comment.WriteString(field)
    }

    length := comment.Len()
    out = append(out, 0x80|4, byte(length>>16), byte(length>>8), byte(length))
    out = append(out, comment.Bytes()...)
    out = append(out, sample[position:]...)

    if err := os.WriteFile(destination, out, 0o644); err != nil {
        t.Fatal(err)
    }
}

func writeDemoLibrary(t *testing.T, data *userdata.Data, now time.Time) (root string, playing string) {
    t.Helper()

    root = t.TempDir()
    for _, book := range demoBooks {
        folder := filepath.Join(root, book.author, book.title)
        if err := os.MkdirAll(folder, 0o755); err != nil {
            t.Fatal(err)
        }
        if err := os.WriteFile(filepath.Join(folder, "cover.png"), coverArt(book), 0o644); err != nil {
            t.Fatal(err)
        }

        partSeconds := book.hours * 3600 / float64(book.parts)
        var parts []string
        for index := range book.parts {
            part := filepath.Join(folder, fmt.Sprintf("Part %03d.flac", index+1))
            taggedRecording(t, part, map[string]string{
                "TITLE":  fmt.Sprintf("Chapter %d", index+1),
                "ALBUM":  book.title,
                "ARTIST": book.author,
            })

            info, err := os.Stat(part)
            if err != nil {
                t.Fatal(err)
            }
            data.Durations[part] = userdata.PartDuration{Size: info.Size(), Modified: info.ModTime().UnixNano(), Seconds: partSeconds}
            parts = append(parts, part)
        }

        listened := now.Add(-time.Duration(book.hoursAgo) * time.Hour).Unix()
        switch {
        case book.progress >= 1:
            data.Positions[folder] = userdata.Position{Part: parts[0], Listened: listened, Finished: true}
            data.Stats.Complete(now.Add(-time.Duration(book.hoursAgo)*time.Hour), folder)
        case book.progress > 0:
            data.Positions[folder] = userdata.Position{Part: parts[int(book.progress*float64(book.parts))], Position: 0.5, Listened: listened}
        }

        if book.progress > 0 {
            data.Stats.Books[folder] = min(book.progress, 1) * book.hours * 3600 / 1.15
        }
        if book.title == playingBook {
            playing = folder
        }
    }

    return root, playing
}

func writeDemoListening(log *stats.Log, now time.Time) {
    seed := uint32(7)
    random := func() float64 {
        seed = seed*1664525 + 1013904223
        return float64(seed>>8) / float64(1<<24)
    }

    for daysAgo := range 150 {
        minutes := 15 + random()*80
        if daysAgo > 12 && random() < 0.3 {
            continue
        }
        if time.Now().AddDate(0, 0, -daysAgo).Weekday() == time.Saturday {
            minutes *= 1.6
        }

        key := now.AddDate(0, 0, -daysAgo).Format("2006-01-02")
        log.Days[key] = stats.Day{Listened: minutes * 60, Content: minutes * 60 * 1.15}
    }
}

var demoCollections = map[string][]string{
    "Currently reading": {"Frankenstein", "Pride and Prejudice", "Moby-Dick"},
    "Classics":          {"Frankenstein", "Pride and Prejudice", "Moby-Dick", "Dracula", "Little Women"},
    "For bedtime":       {"Little Women", "Treasure Island", "The Adventures of Sherlock Holmes", "The Time Machine"},
}

func writeDemoCollections(data *userdata.Data, root string) {
    for name, titles := range demoCollections {
        data.CreateCollection(name)
        for _, title := range titles {
            for _, book := range demoBooks {
                if book.title == title {
                    data.SetInCollection(name, filepath.Join(root, book.author, book.title), true)
                }
            }
        }
    }
}

func demoUI(t *testing.T, variant fyne.ThemeVariant) *UI {
    t.Helper()

    a := test.NewApp()
    t.Cleanup(a.Quit)
    a.Settings().SetTheme(kit.Theme{Forced: true, Variant: variant})

    window := a.NewWindow("Audiobook Player")
    window.Canvas().(test.WindowlessCanvas).SetScale(screenshotScale)
    window.Resize(fyne.NewSize(620, 780))

    ui := &UI{
        App:    a,
        Window: window,
        data:   userdata.New(userdata.FileIn(t.TempDir())),
        async: func(work func()) {
            work()
        },
    }

    now := time.Now()
    root, playing := writeDemoLibrary(t, ui.data, now)
    writeDemoListening(ui.data.Stats, now)
    writeDemoCollections(ui.data, root)

    ui.build()
    window.SetContent(ui.content)
    ui.shelf.AddFrom([]string{root})
    if err := ui.playFromShelf(playing); err != nil {
        t.Fatal(err)
    }

    return ui
}

func saveScreenshot(t *testing.T, ui *UI, name string) {
    t.Helper()

    var encoded bytes.Buffer
    if err := png.Encode(&encoded, ui.Window.Canvas().Capture()); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(filepath.Join(screenshotFolder, name+".png"), encoded.Bytes(), 0o644); err != nil {
        t.Fatal(err)
    }
}

func TestScreenshots(t *testing.T) {
    if err := os.MkdirAll(screenshotFolder, 0o755); err != nil {
        t.Fatal(err)
    }

    for _, variant := range []struct {
        name  string
        theme fyne.ThemeVariant
        tabs  []string
    }{
        {"light", theme.VariantLight, []string{"now-playing", "library", "collection"}},
        {"dark", theme.VariantDark, []string{"now-playing", "statistics"}},
    } {
        ui := demoUI(t, variant.theme)
        tabs := map[string]*container.TabItem{
            "now-playing": ui.playerTab,
            "library":     ui.libraryTab,
            "collection":  ui.libraryTab,
            "statistics":  ui.statsTab,
        }

        for _, name := range variant.tabs {
            ui.tabs.Select(tabs[name])
            if name == "collection" {
                ui.shelf.ShowCollection("For bedtime")
            }
            saveScreenshot(t, ui, name+"-"+variant.name)
        }
        ui.App.Quit()
    }
}
