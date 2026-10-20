package statistics

import (
    "fmt"
    "path/filepath"
    "strconv"
    "strings"
    "time"

    "fyne.io/fyne/v2"
    "fyne.io/fyne/v2/canvas"
    "fyne.io/fyne/v2/container"
    "fyne.io/fyne/v2/layout"
    "fyne.io/fyne/v2/theme"
    "fyne.io/fyne/v2/widget"

    "github.com/izzzicos/audiobook-player/library"
    "github.com/izzzicos/audiobook-player/stats"
    "github.com/izzzicos/audiobook-player/ui/kit"
    "github.com/izzzicos/audiobook-player/ui/userdata"
)

const (
    statsDays        = 30
    statsWeeks       = 12
    statsCalendar    = 26
    statsTopBooks    = 5
    maxListeningTick = 5 * time.Second
)

type statTile struct {
    value *widget.Label
    note  *widget.Label
}

type View struct {
    data     *userdata.Data
    title    func(key string) (string, bool)
    lastTick time.Time

    total, week, average, finished, streak, speed statTile

    content fyne.CanvasObject
    empty   fyne.CanvasObject
    scroll  *container.Scroll

    daily, weekly          *columnChart
    calendar               *heatmap
    dailyNote, weeklyNote  *widget.Label
    calendarNote, topNote  *widget.Label
    dailyTable, weekTable  *fyne.Container
    calendarTable, topList *fyne.Container
}

func (v *View) Record(now time.Time, playing bool, key string, speed float64) {
    previous := v.lastTick
    v.lastTick = now

    if previous.IsZero() || !playing || key == "" {
        return
    }

    elapsed := now.Sub(previous)
    if elapsed <= 0 || elapsed > maxListeningTick {
        return
    }

    v.data.Stats.Record(now, elapsed.Seconds(), speed, key)
}

func newStatTile(label string) (fyne.CanvasObject, statTile) {
    caption := widget.NewLabel(label)
    caption.SizeName = theme.SizeNameCaptionText
    caption.Importance = widget.LowImportance

    value := widget.NewLabel("0m")
    value.TextStyle = fyne.TextStyle{Bold: true}
    value.SizeName = theme.SizeNameHeadingText
    value.Truncation = fyne.TextTruncateEllipsis

    note := widget.NewLabel(" ")
    note.SizeName = theme.SizeNameCaptionText
    note.Importance = widget.LowImportance
    note.Truncation = fyne.TextTruncateEllipsis

    background := canvas.NewRectangle(theme.Color(theme.ColorNameInputBackground))
    background.CornerRadius = theme.InputRadiusSize()
    kit.FollowTheme(func() {
        background.FillColor = theme.Color(theme.ColorNameInputBackground)
        background.Refresh()
    })

    content := container.New(
        layout.NewCustomPaddedVBoxLayout(-theme.Padding()*2),
        caption,
        value,
        note,
    )

    return container.NewStack(background, container.New(layout.NewCustomPaddedLayout(4, 4, 6, 6), content)),
        statTile{value: value, note: note}
}

func (v *View) chartSection(title string, note *widget.Label, chart, table fyne.CanvasObject) fyne.CanvasObject {
    heading := widget.NewLabel(title)
    heading.TextStyle = fyne.TextStyle{Bold: true}
    heading.SizeName = theme.SizeNameSubHeadingText

    var toggle *widget.Button
    toggle = widget.NewButtonWithIcon("Table", theme.ListIcon(), func() {
        if table.Visible() {
            table.Hide()
            chart.Show()
            toggle.SetText("Table")
            toggle.SetIcon(theme.ListIcon())
        } else {
            chart.Hide()
            table.Show()
            toggle.SetText("Chart")
            toggle.SetIcon(theme.NewThemedResource(kit.StatisticsIcon))
        }
        v.scroll.Refresh()
    })
    toggle.Importance = widget.LowImportance
    table.Hide()

    return container.NewVBox(
        container.NewBorder(nil, nil, nil, toggle, heading),
        note,
        container.NewStack(chart, table),
    )
}

func newNote() *widget.Label {
    note := widget.NewLabel("")
    note.Importance = widget.LowImportance
    note.Wrapping = fyne.TextWrapWord
    return note
}

func New(data *userdata.Data, title func(key string) (string, bool)) *View {
    v := &View{data: data, title: title}

    totalTile, total := newStatTile("Total listening")
    weekTile, week := newStatTile("This week")
    averageTile, average := newStatTile("Daily average")
    finishedTile, finished := newStatTile("Books finished")
    streakTile, streak := newStatTile("Current streak")
    speedTile, speed := newStatTile("Average speed")
    v.total, v.week, v.average, v.finished, v.streak, v.speed = total, week, average, finished, streak, speed

    tiles := container.NewGridWithColumns(3, totalTile, weekTile, averageTile, finishedTile, streakTile, speedTile)

    v.daily = newColumnChart()
    v.weekly = newColumnChart()
    v.calendar = newHeatmap()
    v.dailyNote, v.weeklyNote, v.calendarNote, v.topNote = newNote(), newNote(), newNote(), newNote()
    v.dailyTable = container.NewVBox()
    v.weekTable = container.NewVBox()
    v.calendarTable = container.NewVBox()
    v.topList = container.NewVBox()

    v.content = container.NewVBox(
        kit.Gap(),
        v.chartSection("Daily activity", v.dailyNote, v.daily, v.dailyTable),
        kit.Gap(),
        v.chartSection("Weekly activity", v.weeklyNote, v.weekly, v.weekTable),
        kit.Gap(),
        v.chartSection(
            "Listening calendar",
            v.calendarNote,
            container.NewVBox(v.calendar, newHeatmapLegend()),
            v.calendarTable,
        ),
        kit.Gap(),
        kit.Heading("Most listened"),
        v.topNote,
        v.topList,
    )

    emptyText := widget.NewLabel("Your listening statistics will appear here once you start listening to a book.")
    emptyText.Alignment = fyne.TextAlignCenter
    emptyText.Wrapping = fyne.TextWrapWord
    emptyText.Importance = widget.LowImportance
    v.empty = container.NewVBox(kit.Gap(), kit.Gap(), emptyText)

    v.scroll = container.NewVScroll(container.New(
        layout.NewCustomPaddedLayout(12, 24, 16, 16),
        container.NewVBox(tiles, v.empty, v.content),
    ))

    return v
}

func (v *View) Content() fyne.CanvasObject {
    return v.scroll
}

func formatSpeed(speed float64) string {
    if speed <= 0 {
        return "–"
    }

    text := strconv.FormatFloat(speed, 'f', 2, 64)
    text = strings.TrimSuffix(text, "0")
    return text + "×"
}

func days(count int) string {
    return kit.CountPhrase(count, "day", "days", "0 days")
}

func (v *View) bookTitle(key string) string {
    if v.title != nil {
        if title, found := v.title(key); found {
            return title
        }
    }

    return library.CleanName(strings.TrimSuffix(filepath.Base(key), filepath.Ext(key)))
}

func tableRow(label, value string) fyne.CanvasObject {
    name := widget.NewLabel(label)
    amount := widget.NewLabel(value)
    amount.TextStyle = fyne.TextStyle{Bold: true}
    return container.NewBorder(nil, nil, nil, amount, name)
}

func fillTable(table *fyne.Container, periods []stats.Period, label func(stats.Period) string) {
    var rows []fyne.CanvasObject
    for i := len(periods) - 1; i >= 0; i-- {
        rows = append(rows, tableRow(label(periods[i]), kit.FormatListening(periods[i].Seconds)))
    }

    table.Objects = rows
    table.Refresh()
}

func (v *View) Refresh() {
    log := v.data.Stats
    now := time.Now()
    today := stats.DayStart(now)
    current, longest := log.Streaks(now)

    v.total.value.SetText(kit.FormatListening(log.Total()))
    v.total.note.SetText(fmt.Sprintf("across %s", kit.CountPhrase(len(log.Books), "book", "books", "no books")))
    v.week.value.SetText(kit.FormatListening(log.ThisWeek(now)))
    v.week.note.SetText("since Monday")
    v.average.value.SetText(kit.FormatListening(log.DailyAverage(now, statsDays)))
    v.average.note.SetText(fmt.Sprintf("over the last %d days", statsDays))
    v.finished.value.SetText(strconv.Itoa(log.CompletedCount()))
    v.finished.note.SetText("listened to the end")
    v.streak.value.SetText(days(current))
    v.streak.note.SetText("longest " + days(longest))
    v.speed.value.SetText(formatSpeed(log.AverageSpeed()))
    v.speed.note.SetText("weighted by listening time")

    defer v.scroll.Refresh()

    if log.Total() == 0 {
        v.empty.Show()
        v.content.Hide()
        return
    }
    v.empty.Hide()
    v.content.Show()

    daily := log.Daily(now, statsDays)
    values, ticks, details := make([]float64, len(daily)), make([]string, len(daily)), make([]string, len(daily))
    dailyTotal := 0.0
    for i, day := range daily {
        values[i] = day.Seconds
        dailyTotal += day.Seconds
        details[i] = day.Start.Format("Mon 2 Jan")
        if (len(daily)-1-i)%7 == 0 {
            ticks[i] = day.Start.Format("2 Jan")
        }
    }
    ticks[len(ticks)-1] = "Today"
    details[len(details)-1] = "Today"
    v.daily.SetData(values, ticks, details)
    v.dailyNote.SetText(fmt.Sprintf("Last %d days · %s in total", statsDays, kit.FormatListening(dailyTotal)))
    fillTable(v.dailyTable, daily, func(day stats.Period) string {
        if day.Start.Equal(today) {
            return "Today"
        }
        return day.Start.Format("Mon 2 Jan")
    })

    weekly := log.Weekly(now, statsWeeks)
    values, ticks, details = make([]float64, len(weekly)), make([]string, len(weekly)), make([]string, len(weekly))
    weeklyTotal := 0.0
    for i, week := range weekly {
        values[i] = week.Seconds
        weeklyTotal += week.Seconds
        details[i] = "Week of " + week.Start.Format("2 Jan")
        if (len(weekly)-1-i)%3 == 0 {
            ticks[i] = week.Start.Format("2 Jan")
        }
    }
    ticks[len(ticks)-1] = "This week"
    details[len(details)-1] = "This week"
    v.weekly.SetData(values, ticks, details)
    v.weeklyNote.SetText(fmt.Sprintf("Last %d weeks · %s in total", statsWeeks, kit.FormatListening(weeklyTotal)))
    fillTable(v.weekTable, weekly, func(week stats.Period) string {
        return "Week of " + week.Start.Format("2 Jan 2006")
    })

    first := stats.WeekStart(now).AddDate(0, 0, -7*(statsCalendar-1))
    calendarDays := int(today.Sub(first).Hours()/24+0.5) + 1
    calendar := log.Daily(now, calendarDays)
    v.calendar.SetData(calendar)
    v.calendarNote.SetText(fmt.Sprintf(
        "Last %d weeks · current streak %s, longest %s. A day counts towards a streak after a minute of listening.",
        statsCalendar, days(current), days(longest),
    ))

    var listenedDays []stats.Period
    for _, day := range calendar {
        if day.Seconds > 0 {
            listenedDays = append(listenedDays, day)
        }
    }
    fillTable(v.calendarTable, listenedDays, func(day stats.Period) string {
        return day.Start.Format("Mon 2 Jan 2006")
    })

    top := log.TopBooks(statsTopBooks)
    var rows []fyne.CanvasObject
    for _, book := range top {
        title := widget.NewLabel(v.bookTitle(book.Key))
        title.Truncation = fyne.TextTruncateEllipsis
        rows = append(rows, container.New(
            layout.NewCustomPaddedVBoxLayout(-theme.Padding()),
            title,
            newValueBar(book.Seconds/top[0].Seconds, kit.FormatListening(book.Seconds)),
        ))
    }
    v.topList.Objects = rows
    v.topList.Refresh()
    v.topNote.SetText("By time spent listening")
}
