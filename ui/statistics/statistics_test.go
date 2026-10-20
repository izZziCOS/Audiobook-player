package statistics

import (
    "math"
    "strings"
    "testing"
    "time"

    "fyne.io/fyne/v2"
    "fyne.io/fyne/v2/driver/desktop"
    "fyne.io/fyne/v2/test"
    "fyne.io/fyne/v2/widget"

    "github.com/izzzicos/audiobook-player/stats"
    "github.com/izzzicos/audiobook-player/ui/userdata"
)

func newTestView(t *testing.T, log *stats.Log) *View {
    t.Helper()

    app := test.NewApp()
    t.Cleanup(app.Quit)

    data := userdata.New("")
    data.Stats = log
    v := New(data, func(key string) (string, bool) {
        return map[string]string{"/books/Dune.m4b": "Dune"}[key], key == "/books/Dune.m4b"
    })

    window := app.NewWindow("Statistics")
    window.SetContent(v.Content())
    window.Resize(fyne.NewSize(620, 900))
    v.Refresh()

    return v
}

func seededLog(now time.Time) *stats.Log {
    log := stats.New()
    for daysAgo, minutes := range map[int]float64{0: 25, 1: 40, 2: 70, 3: 5, 9: 120, 40: 90} {
        key := now.AddDate(0, 0, -daysAgo).Format("2006-01-02")
        log.Days[key] = stats.Day{Listened: minutes * 60, Content: minutes * 60 * 1.2}
    }
    log.Books["/books/Dune.m4b"] = 200 * 60
    log.Books["/books/The Hobbit"] = 150 * 60
    log.Complete(now, "/books/Dune.m4b")
    return log
}

func TestListeningIsRecordedOnlyWhilePlaying(t *testing.T) {
    v := newTestView(t, stats.New())
    log := v.data.Stats
    start := time.Now()
    book := "/books/Dune.m4b"

    v.Record(start, true, book, 1)
    v.Record(start.Add(250*time.Millisecond), true, book, 1)
    v.Record(start.Add(time.Second), true, book, 1)
    if total := log.Total(); math.Abs(total-1) > 1e-9 {
        t.Fatalf("recorded %v seconds, want 1", total)
    }

    v.Record(start.Add(time.Minute), true, book, 1)
    if total := log.Total(); math.Abs(total-1) > 1e-9 {
        t.Fatalf("a long gap between ticks (the computer slept) should not count, recorded %v", total)
    }

    v.Record(start.Add(time.Minute+time.Second), false, book, 1)
    if total := log.Total(); math.Abs(total-1) > 1e-9 {
        t.Fatalf("paused time should not count, recorded %v", total)
    }

    v.Record(start.Add(time.Minute+2*time.Second), true, "", 1)
    if total := log.Total(); math.Abs(total-1) > 1e-9 {
        t.Fatalf("time with no book open should not count, recorded %v", total)
    }

    v.Record(start.Add(time.Minute+3*time.Second), true, book, 1.5)
    if speed := log.AverageSpeed(); math.Abs(speed-1.25) > 1e-9 {
        t.Fatalf("average speed %v, want 1.25", speed)
    }
    if log.Books[book] != 2 {
        t.Fatalf("time should be recorded against the book, got %v", log.Books)
    }
}

func TestMostListenedUsesLibraryTitles(t *testing.T) {
    v := newTestView(t, seededLog(time.Now()))

    var titles []string
    for _, object := range test.LaidOutObjects(v.topList) {
        if label, ok := object.(*widget.Label); ok {
            titles = append(titles, label.Text)
        }
    }

    if strings.Join(titles, "|") != "Dune|The Hobbit" {
        t.Fatalf("most listened shows %v", titles)
    }
}

func TestStatisticsTab(t *testing.T) {
    now := time.Now()
    v := newTestView(t, seededLog(now))

    if v.total.value.Text != "5h 50m" || v.finished.value.Text != "1" || v.speed.value.Text != "1.2×" {
        t.Fatalf("tiles: total %q, finished %q, speed %q", v.total.value.Text, v.finished.value.Text, v.speed.value.Text)
    }
    if v.streak.value.Text != "4 days" || v.streak.note.Text != "longest 4 days" {
        t.Fatalf("streak tile %q / %q", v.streak.value.Text, v.streak.note.Text)
    }
    if !v.content.Visible() || v.empty.Visible() {
        t.Fatal("charts should be shown once there is listening")
    }

    if len(v.daily.values) != statsDays || v.daily.values[statsDays-1] != 25*60 || v.daily.ticks[statsDays-1] != "Today" {
        t.Fatalf("daily chart: %d values, last %v, tick %q", len(v.daily.values), v.daily.values[statsDays-1], v.daily.ticks[statsDays-1])
    }
    if len(v.dailyTable.Objects) != statsDays {
        t.Fatalf("daily table has %d rows", len(v.dailyTable.Objects))
    }

    if len(v.weekly.values) != statsWeeks {
        t.Fatalf("weekly chart has %d values", len(v.weekly.values))
    }
    weeklyTotal := 0.0
    for _, value := range v.weekly.values {
        weeklyTotal += value
    }
    if weeklyTotal != (25+40+70+5+120)*60 && weeklyTotal != (25+40+70+5+120+90)*60 {
        t.Fatalf("weekly total %v", weeklyTotal)
    }

    days := v.calendar.days
    if len(days) < 7*(statsCalendar-1)+1 || days[0].Start.Weekday() != time.Monday || !days[len(days)-1].Start.Equal(stats.DayStart(now)) {
        t.Fatalf("calendar should run from a Monday to today, got %d days from %v", len(days), days[0].Start)
    }
    if len(v.calendarTable.Objects) != 6 {
        t.Fatalf("the calendar table should list the 6 days with listening in range, got %d", len(v.calendarTable.Objects))
    }

    if len(v.topList.Objects) != 2 {
        t.Fatalf("most listened shows %d books", len(v.topList.Objects))
    }

    v.daily.Resize(fyne.NewSize(520, 220))
    v.daily.MouseMoved(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(519, 150)}})
    if v.daily.hovered != statsDays-1 {
        t.Fatalf("hovering the last column should select today, got %d", v.daily.hovered)
    }
    v.daily.MouseOut()
    if v.daily.hovered != -1 {
        t.Fatal("leaving the chart should clear the hover")
    }

    v.calendar.Resize(fyne.NewSize(520, 220))
    v.calendar.MouseMoved(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(v.calendar.left+1, v.calendar.top+1)}})
    if v.calendar.hovered != 0 {
        t.Fatalf("hovering the first cell should select it, got %d", v.calendar.hovered)
    }
}

func TestTableView(t *testing.T) {
    v := newTestView(t, seededLog(time.Now()))

    var toggle *widget.Button
    for _, object := range test.LaidOutObjects(v.content) {
        if button, ok := object.(*widget.Button); ok && button.Text == "Table" {
            toggle = button
            break
        }
    }
    if toggle == nil {
        t.Fatal("no table toggle found")
    }

    test.Tap(toggle)
    if !v.dailyTable.Visible() || v.daily.Visible() || toggle.Text != "Chart" {
        t.Fatal("the toggle should swap the daily chart for its table")
    }

    driver := fyne.CurrentApp().Driver()
    tableBottom := driver.AbsolutePositionForObject(v.dailyTable).Y + v.dailyTable.MinSize().Height
    if next := driver.AbsolutePositionForObject(v.weeklyNote).Y; next < tableBottom {
        t.Fatalf("the next section starts at %v, inside the table that ends at %v", next, tableBottom)
    }

    test.Tap(toggle)
    if v.dailyTable.Visible() || !v.daily.Visible() {
        t.Fatal("tapping again should bring the chart back")
    }
}

func TestEmptyStatistics(t *testing.T) {
    v := newTestView(t, stats.New())

    if v.content.Visible() || !v.empty.Visible() {
        t.Fatal("with no listening the empty message should be shown")
    }
    if v.total.value.Text != "0m" || v.speed.value.Text != "–" {
        t.Fatalf("empty tiles %q %q", v.total.value.Text, v.speed.value.Text)
    }
}

func TestChartTicks(t *testing.T) {
    for maximum, want := range map[float64]string{
        0:     "0|1m",
        200:   "0|1m|2m|3m|4m",
        2500:  "0|15m|30m|45m",
        5000:  "0|30m|1h|1h 30m",
        30000: "0|3h|6h|9h",
    } {
        var labels []string
        for _, tick := range chartTicks(maximum) {
            labels = append(labels, chartTickLabel(tick))
        }

        if got := strings.Join(labels, "|"); got != want {
            t.Errorf("ticks for %v: %q, want %q", maximum, got, want)
        }
    }

    for seconds, level := range map[float64]int{0: 0, 30: 1, 900: 1, 901: 2, 1800: 2, 3000: 3, 3601: 4} {
        if got := heatmapLevel(seconds); got != level {
            t.Errorf("heatmapLevel(%v) = %d, want %d", seconds, got, level)
        }
    }
}
