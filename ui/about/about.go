package about

import (
    "context"
    "fmt"
    "image"
    "net/url"
    "regexp"
    "strings"
    "time"

    "fyne.io/fyne/v2"
    "fyne.io/fyne/v2/canvas"
    "fyne.io/fyne/v2/container"
    "fyne.io/fyne/v2/dialog"
    "fyne.io/fyne/v2/theme"
    "fyne.io/fyne/v2/widget"

    "github.com/izzzicos/audiobook-player/library"
    "github.com/izzzicos/audiobook-player/online"
    "github.com/izzzicos/audiobook-player/ui/kit"
    "github.com/izzzicos/audiobook-player/ui/userdata"
)

const (
    preferenceLookups = "audibleLookups"
    preferenceStore   = "audibleStore"
    coverSide         = 120
    privacyNote       = "When you look a book up, the title and author you search for are sent to Audible. " +
        "Nothing is sent until you turn this on, and only when you search for a book or open the details of one you've matched."
)

type Book struct {
    Title  string
    Author string
    Cover  image.Image
}

type About struct {
    OnLinked func(key string)

    window fyne.Window
    prefs  fyne.Preferences
    data   *userdata.Data
    async  func(func())
    client *online.Client
    now    func() time.Time

    section     fyne.CanvasObject
    enabled     *widget.Check
    storeSelect *widget.Select
}

func New(window fyne.Window, prefs fyne.Preferences, data *userdata.Data, async func(func())) *About {
    a := &About{
        window: window,
        prefs:  prefs,
        data:   data,
        async:  async,
        client: online.NewClient(),
        now:    time.Now,
    }
    a.buildSection()

    return a
}

func (a *About) Section() fyne.CanvasObject {
    return a.section
}

func (a *About) Enabled() bool {
    return a.prefs.Bool(preferenceLookups)
}

func (a *About) SetEnabled(enabled bool) {
    a.prefs.SetBool(preferenceLookups, enabled)
    a.enabled.SetChecked(enabled)
}

func (a *About) store() online.Store {
    return online.StoreFor(a.prefs.String(preferenceStore))
}

func (a *About) buildSection() {
    a.enabled = widget.NewCheck("Look up ratings and descriptions on Audible", func(enabled bool) {
        a.prefs.SetBool(preferenceLookups, enabled)
    })
    a.enabled.SetChecked(a.Enabled())

    var names []string
    for _, store := range online.Stores {
        names = append(names, store.Name)
    }
    a.storeSelect = widget.NewSelect(names, func(name string) {
        a.prefs.SetString(preferenceStore, online.StoreNamed(name).Code)
    })
    a.storeSelect.SetSelected(a.store().Name)

    a.section = kit.Section(
        "Ratings and descriptions",
        "Right-click a book and choose About this book to see its Audible rating, how many people rated it, "+
            "its description and release date. "+privacyNote,
        a.enabled,
        container.NewBorder(nil, nil, widget.NewLabel("Audible store"), nil, a.storeSelect),
    )
}

type view struct {
    about *About
    key   string
    book  Book

    rating      *widget.Label
    facts       *widget.Label
    description *widget.Label
    message     *widget.Label
    primary     *widget.Button
    openLink    *widget.Button
    unlink      *widget.Button
    popUp       dialog.Dialog
}

func (a *About) Show(key string, book Book) {
    v := a.open(key, book)
    v.popUp.Show()
}

func (a *About) open(key string, book Book) *view {
    v := &view{about: a, key: key, book: book}
    v.build()
    v.refreshIfStale()

    return v
}

func (v *view) build() {
    cover := canvas.NewImageFromImage(v.book.Cover)
    cover.FillMode = canvas.ImageFillContain
    cover.CornerRadius = 10
    cover.SetMinSize(fyne.NewSquareSize(coverSide))
    var coverBox fyne.CanvasObject = cover
    if v.book.Cover == nil {
        background := canvas.NewRectangle(theme.Color(theme.ColorNameInputBackground))
        background.CornerRadius = 10
        background.SetMinSize(fyne.NewSquareSize(coverSide))
        icon := canvas.NewImageFromResource(theme.NewColoredResource(theme.MediaMusicIcon(), theme.ColorNamePlaceHolder))
        icon.SetMinSize(fyne.NewSquareSize(coverSide * 0.4))
        coverBox = container.NewStack(background, container.NewCenter(icon))
    }

    title := widget.NewLabel(v.book.Title)
    title.TextStyle = fyne.TextStyle{Bold: true}
    title.SizeName = theme.SizeNameSubHeadingText
    title.Wrapping = fyne.TextWrapWord

    author := widget.NewLabel(v.book.Author)
    author.Wrapping = fyne.TextWrapWord
    if v.book.Author == "" {
        author.Hide()
    }

    v.rating = widget.NewLabel("")
    v.rating.TextStyle = fyne.TextStyle{Bold: true}
    v.rating.Wrapping = fyne.TextWrapWord

    v.facts = widget.NewLabel("")
    v.facts.Wrapping = fyne.TextWrapWord
    v.facts.SizeName = theme.SizeNameCaptionText
    v.facts.Importance = widget.LowImportance

    v.description = widget.NewLabel("")
    v.description.Wrapping = fyne.TextWrapWord

    v.message = widget.NewLabel("")
    v.message.Wrapping = fyne.TextWrapWord
    v.message.Importance = widget.LowImportance

    v.primary = widget.NewButtonWithIcon("", theme.SearchIcon(), nil)
    v.openLink = widget.NewButtonWithIcon("Open on Audible", theme.ComputerIcon(), v.openOnAudible)
    v.unlink = widget.NewButtonWithIcon("Not this book", theme.ContentClearIcon(), func() {
        v.about.data.UnlinkOnline(v.key)
        v.changed()
    })
    v.unlink.Importance = widget.LowImportance

    var popUp dialog.Dialog
    closeButton := widget.NewButtonWithIcon("Close", theme.CancelIcon(), func() {
        popUp.Hide()
    })

    header := container.NewBorder(nil, nil, container.NewVBox(coverBox), nil, container.NewVBox(title, author, v.rating, v.facts))
    descriptionScroll := container.NewVScroll(v.description)
    descriptionScroll.SetMinSize(fyne.NewSize(0, 200))

    popUp = dialog.NewCustomWithoutButtons("About this book", container.NewBorder(
        container.NewVBox(header, widget.NewSeparator()),
        container.NewVBox(
            v.message,
            container.NewHBox(v.primary, v.openLink, v.unlink),
            closeButton,
        ),
        nil,
        nil,
        descriptionScroll,
    ), v.about.window)
    v.popUp = popUp

    v.show()

    size := v.about.window.Canvas().Size()
    popUp.Resize(fyne.NewSize(min(size.Width-40, 560), min(size.Height-40, max(popUp.MinSize().Height, 520))))
}

func (v *view) show() {
    a := v.about
    details, linked := a.data.Online(v.key)

    v.rating.SetText("")
    v.facts.SetText("")
    v.description.SetText("")
    v.openLink.Hide()
    v.unlink.Hide()

    switch {
    case linked:
        edition := details.Edition
        v.rating.SetText(edition.RatingText())
        v.facts.SetText(facts(edition))
        v.description.SetText(edition.Description)
        v.message.SetText(fmt.Sprintf("From Audible's %s store, updated %s.", online.StoreFor(details.Store).Name, fetchedDate(details.Fetched)))
        v.openLink.Show()
        v.unlink.Show()
        v.setPrimary("Change edition…", v.find)
    case a.Enabled():
        v.message.SetText("Find this book on Audible to see its rating, how many people rated it, its description and release date.")
        v.setPrimary("Find on Audible…", v.find)
    default:
        v.message.SetText("Audible look-ups are off. " + privacyNote)
        v.setPrimary("Turn on Audible look-ups", func() {
            a.SetEnabled(true)
            v.show()
            v.find()
        })
    }
}

func (v *view) setPrimary(text string, action func()) {
    v.primary.SetText(text)
    v.primary.OnTapped = action
    v.primary.Importance = widget.HighImportance
    v.primary.Refresh()
}

func (v *view) changed() {
    v.show()
    if v.about.OnLinked != nil {
        v.about.OnLinked(v.key)
    }
}

func (v *view) refreshIfStale() {
    a := v.about
    details, linked := a.data.Online(v.key)
    if !linked || !a.Enabled() || !details.Stale(a.now()) {
        return
    }

    store := online.StoreFor(details.Store)
    asin := details.Edition.ASIN
    a.async(func() {
        edition, err := a.client.Lookup(context.Background(), store, asin)

        fyne.Do(func() {
            if err != nil {
                fyne.LogError("Updating a book's Audible details failed", err)
                return
            }
            if current, still := a.data.Online(v.key); still && current.Edition.ASIN == asin {
                a.data.UpdateOnline(store.Code, edition)
                v.changed()
            }
        })
    })
}

func (v *view) openOnAudible() {
    details, linked := v.about.data.Online(v.key)
    if !linked {
        return
    }

    link, err := url.Parse(details.Edition.Link(online.StoreFor(details.Store)))
    if err == nil {
        err = fyne.CurrentApp().OpenURL(link)
    }
    if err != nil {
        dialog.ShowError(err, v.about.window)
    }
}

func (v *view) find() {
    a := v.about
    a.openFinder(v.key, v.book, func(store online.Store, chosen online.Edition) {
        v.message.SetText("Getting the full details from Audible…")
        a.async(func() {
            edition, err := a.client.Lookup(context.Background(), store, chosen.ASIN)
            if err != nil {
                fyne.LogError("Getting a book's full Audible details failed, so the search result is used", err)
                edition = chosen
            }

            fyne.Do(func() {
                a.data.LinkOnline(v.key, store.Code, edition)
                v.changed()
            })
        })
    })
}

func facts(edition online.Edition) string {
    var parts []string
    if len(edition.Narrators) > 0 {
        parts = append(parts, "Narrated by "+strings.Join(edition.Narrators, ", "))
    }
    if released, err := time.Parse("2006-01-02", edition.Released); err == nil {
        parts = append(parts, "Released "+released.Format("2 January 2006"))
    }
    if edition.Minutes > 0 {
        parts = append(parts, kit.FormatLength(float64(edition.Minutes*60)))
    }

    return strings.Join(parts, " · ")
}

func fetchedDate(fetched int64) string {
    return time.Unix(fetched, 0).Format("2 January 2006")
}

type finder struct {
    about   *About
    key     string
    store   online.Store
    chosen  func(online.Store, online.Edition)
    title   *widget.Entry
    author  *widget.Entry
    search  *widget.Button
    status  *widget.Label
    list    *widget.List
    results []result
    popUp   dialog.Dialog
}

type result struct {
    edition online.Edition
    tagged  bool
}

var noise = regexp.MustCompile(`(?i)\s*[\(\[][^\)\]]*\b(unabridged|abridged|audio ?book|audible)\b[^\)\]]*[\)\]]`)

func searchTitle(title string) string {
    return strings.TrimSpace(noise.ReplaceAllString(title, ""))
}

func (a *About) openFinder(key string, book Book, chosen func(online.Store, online.Edition)) *finder {
    f := &finder{about: a, key: key, store: a.store(), chosen: chosen}

    f.title = widget.NewEntry()
    f.title.SetText(searchTitle(book.Title))
    f.title.PlaceHolder = "Title"
    f.author = widget.NewEntry()
    f.author.SetText(book.Author)
    f.author.PlaceHolder = "Author"
    f.title.OnSubmitted = func(string) { f.run(false) }
    f.author.OnSubmitted = func(string) { f.run(false) }

    f.search = widget.NewButtonWithIcon("Search", theme.SearchIcon(), func() { f.run(false) })
    f.search.Importance = widget.HighImportance

    f.status = widget.NewLabel("")
    f.status.Wrapping = fyne.TextWrapWord
    f.status.Importance = widget.LowImportance

    f.list = widget.NewList(
        func() int {
            return len(f.results)
        },
        func() fyne.CanvasObject {
            name := widget.NewLabel("")
            name.TextStyle = fyne.TextStyle{Bold: true}
            name.Truncation = fyne.TextTruncateEllipsis
            people := widget.NewLabel("")
            people.SizeName = theme.SizeNameCaptionText
            people.Truncation = fyne.TextTruncateEllipsis
            rating := widget.NewLabel("")
            rating.SizeName = theme.SizeNameCaptionText
            rating.Importance = widget.LowImportance
            return container.NewVBox(name, people, rating)
        },
        func(id widget.ListItemID, object fyne.CanvasObject) {
            labels := object.(*fyne.Container).Objects
            row := f.results[id]
            labels[0].(*widget.Label).SetText(row.heading())
            labels[1].(*widget.Label).SetText(row.people())
            labels[2].(*widget.Label).SetText(row.edition.RatingText())
        },
    )
    f.list.OnSelected = func(id widget.ListItemID) {
        f.popUp.Hide()
        f.chosen(f.store, f.results[id].edition)
    }

    form := widget.NewForm(widget.NewFormItem("Title", f.title), widget.NewFormItem("Author", f.author))
    f.popUp = dialog.NewCustom(
        "Find on Audible ("+f.store.Name+")",
        "Cancel",
        container.NewBorder(
            container.NewVBox(form, f.search, f.status),
            nil,
            nil,
            nil,
            f.list,
        ),
        a.window,
    )

    size := a.window.Canvas().Size()
    f.popUp.Resize(fyne.NewSize(min(size.Width-40, 560), size.Height*0.85))
    f.popUp.Show()
    f.run(true)

    return f
}

func (r result) heading() string {
    heading := r.edition.Title
    if r.edition.Subtitle != "" {
        heading += ": " + r.edition.Subtitle
    }
    if r.tagged {
        heading += " · matches this book's files"
    }

    return heading
}

func (r result) people() string {
    var parts []string
    if len(r.edition.Authors) > 0 {
        parts = append(parts, strings.Join(r.edition.Authors, ", "))
    }
    if len(r.edition.Narrators) > 0 {
        parts = append(parts, "narrated by "+strings.Join(r.edition.Narrators, ", "))
    }
    if year := r.edition.Year(); year != "" {
        parts = append(parts, year)
    }

    return strings.Join(parts, " · ")
}

func (f *finder) run(first bool) {
    title, author := f.title.Text, f.author.Text
    if strings.TrimSpace(title) == "" && strings.TrimSpace(author) == "" {
        f.status.SetText("Type a title or an author to search for.")
        return
    }

    f.search.Disable()
    f.status.SetText("Searching Audible…")
    key, store, client := f.key, f.store, f.about.client

    f.about.async(func() {
        var results []result
        if first {
            if asin := taggedASIN(key); asin != "" {
                if edition, err := client.Lookup(context.Background(), store, asin); err == nil {
                    results = append(results, result{edition: edition, tagged: true})
                }
            }
        }

        editions, err := client.Search(context.Background(), store, title, author)
        for _, edition := range editions {
            if len(results) > 0 && results[0].edition.ASIN == edition.ASIN {
                continue
            }
            results = append(results, result{edition: edition})
        }

        fyne.Do(func() {
            f.search.Enable()
            f.results = results
            f.list.UnselectAll()
            f.list.Refresh()

            switch {
            case err != nil && len(results) == 0:
                fyne.LogError("Searching Audible failed", err)
                f.status.SetText(err.Error() + ".")
            case len(results) == 0:
                f.status.SetText("Audible found nothing. Try a shorter title, or leave the author out.")
            case results[0].tagged:
                f.status.SetText("The first edition is the one named in this book's files. The others follow, most rated first.")
            default:
                f.status.SetText("Choose the edition you have. The one most people rated is first, so its rating is the most reliable.")
            }
        })
    })
}

func taggedASIN(key string) string {
    book, err := library.OpenBook(key)
    if err != nil {
        return ""
    }

    return library.ReadMetadata(book.Parts[0]).ASIN
}
