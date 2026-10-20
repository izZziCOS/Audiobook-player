package stats

import (
    "cmp"
    "slices"
    "time"
)

const (
    dateLayout       = "2006-01-02"
    StreakMinimum    = 60.0
    maxRecordedChunk = 5.0
    KeepDays         = 366
)

type Day struct {
    Listened float64 `json:"listened"`
    Content  float64 `json:"content"`
}

type Log struct {
    Days      map[string]Day     `json:"days"`
    Books     map[string]float64 `json:"books"`
    Completed map[string]string  `json:"completed"`
    Earlier   *Summary           `json:"earlier,omitempty"`
}

type Summary struct {
    Listened      float64 `json:"listened"`
    Content       float64 `json:"content"`
    LongestStreak int     `json:"longestStreak"`
    EndingStreak  int     `json:"endingStreak"`
    LastDay       string  `json:"lastDay"`
}

type Period struct {
    Start   time.Time
    Seconds float64
}

type BookTime struct {
    Key     string
    Seconds float64
}

func New() *Log {
    log := &Log{}
    log.ensure()
    return log
}

func (l *Log) ensure() {
    if l.Days == nil {
        l.Days = map[string]Day{}
    }
    if l.Books == nil {
        l.Books = map[string]float64{}
    }
    if l.Completed == nil {
        l.Completed = map[string]string{}
    }
}

func dayKey(t time.Time) string {
    return t.Format(dateLayout)
}

func DayStart(t time.Time) time.Time {
    year, month, day := t.Date()
    return time.Date(year, month, day, 0, 0, 0, 0, t.Location())
}

func WeekStart(t time.Time) time.Time {
    day := DayStart(t)
    offset := (int(day.Weekday()) + 6) % 7
    return day.AddDate(0, 0, -offset)
}

func (l *Log) Record(now time.Time, seconds, speed float64, book string) {
    if seconds <= 0 {
        return
    }
    l.ensure()

    seconds = min(seconds, maxRecordedChunk)
    key := dayKey(now)
    day := l.Days[key]
    day.Listened += seconds
    day.Content += seconds * speed
    l.Days[key] = day

    if book != "" {
        l.Books[book] += seconds
    }
}

func (l *Log) Complete(now time.Time, book string) {
    l.ensure()
    l.Completed[book] = dayKey(now)
}

func (l *Log) Listened(t time.Time) float64 {
    return l.Days[dayKey(t)].Listened
}

func (l *Log) earlier() Summary {
    if l.Earlier == nil {
        return Summary{}
    }
    return *l.Earlier
}

func (l *Log) Total() float64 {
    total := l.earlier().Listened
    for _, day := range l.Days {
        total += day.Listened
    }
    return total
}

func (l *Log) AverageSpeed() float64 {
    listened, content := l.earlier().Listened, l.earlier().Content
    for _, day := range l.Days {
        listened += day.Listened
        content += day.Content
    }

    if listened == 0 {
        return 0
    }
    return content / listened
}

func (l *Log) Daily(today time.Time, days int) []Period {
    periods := make([]Period, days)
    first := DayStart(today).AddDate(0, 0, -(days - 1))

    for i := range periods {
        start := first.AddDate(0, 0, i)
        periods[i] = Period{Start: start, Seconds: l.Listened(start)}
    }

    return periods
}

func (l *Log) Weekly(today time.Time, weeks int) []Period {
    periods := make([]Period, weeks)
    first := WeekStart(today).AddDate(0, 0, -7*(weeks-1))

    for i := range periods {
        start := first.AddDate(0, 0, 7*i)
        total := 0.0
        for day := 0; day < 7; day++ {
            total += l.Listened(start.AddDate(0, 0, day))
        }
        periods[i] = Period{Start: start, Seconds: total}
    }

    return periods
}

func (l *Log) ThisWeek(today time.Time) float64 {
    return l.Weekly(today, 1)[0].Seconds
}

func (l *Log) DailyAverage(today time.Time, days int) float64 {
    total := 0.0
    for _, period := range l.Daily(today, days) {
        total += period.Seconds
    }
    return total / float64(days)
}

func (l *Log) counts(t time.Time) bool {
    return l.Listened(t) >= StreakMinimum
}

func (l *Log) Streaks(today time.Time) (int, int) {
    day := DayStart(today)
    if !l.counts(day) {
        day = day.AddDate(0, 0, -1)
    }

    earlier := l.earlier()

    current := 0
    for l.counts(day) {
        current++
        day = day.AddDate(0, 0, -1)
    }
    if dayKey(day) == earlier.LastDay {
        current += earlier.EndingStreak
    }

    var dates []time.Time
    for key, entry := range l.Days {
        if entry.Listened < StreakMinimum {
            continue
        }
        if date, err := time.ParseInLocation(dateLayout, key, today.Location()); err == nil {
            dates = append(dates, date)
        }
    }
    slices.SortFunc(dates, func(a, b time.Time) int { return a.Compare(b) })

    longest, run := earlier.LongestStreak, 0
    for i, date := range dates {
        switch {
        case i > 0 && dates[i-1].AddDate(0, 0, 1).Equal(date):
            run++
        case i == 0 && dayKey(date.AddDate(0, 0, -1)) == earlier.LastDay:
            run = earlier.EndingStreak + 1
        default:
            run = 1
        }
        longest = max(longest, run)
    }

    return current, max(longest, current)
}

func (l *Log) TopBooks(count int) []BookTime {
    books := make([]BookTime, 0, len(l.Books))
    for key, seconds := range l.Books {
        books = append(books, BookTime{Key: key, Seconds: seconds})
    }

    slices.SortFunc(books, func(a, b BookTime) int {
        if order := cmp.Compare(b.Seconds, a.Seconds); order != 0 {
            return order
        }
        return cmp.Compare(a.Key, b.Key)
    })

    return books[:min(count, len(books))]
}

func (l *Log) Rekey(from, to string) {
    l.ensure()

    if seconds, found := l.Books[from]; found {
        l.Books[to] += seconds
        delete(l.Books, from)
    }

    if date, found := l.Completed[from]; found {
        if date > l.Completed[to] {
            l.Completed[to] = date
        }
        delete(l.Completed, from)
    }
}

func (l *Log) CompletedCount() int {
    return len(l.Completed)
}

func (l *Log) Compact(today time.Time, keepDays int) bool {
    cutoff := dayKey(DayStart(today).AddDate(0, 0, -(keepDays - 1)))

    var old []string
    for key := range l.Days {
        if key < cutoff {
            old = append(old, key)
        }
    }
    if len(old) == 0 {
        return false
    }
    slices.Sort(old)

    summary := l.earlier()
    for _, key := range old {
        day := l.Days[key]
        summary.Listened += day.Listened
        summary.Content += day.Content
        delete(l.Days, key)

        if summary.LastDay != "" && key <= summary.LastDay {
            continue
        }

        date, err := time.ParseInLocation(dateLayout, key, today.Location())
        switch {
        case err != nil:
            continue
        case day.Listened < StreakMinimum:
            summary.EndingStreak = 0
        case summary.LastDay == dayKey(date.AddDate(0, 0, -1)):
            summary.EndingStreak++
        default:
            summary.EndingStreak = 1
        }
        summary.LongestStreak = max(summary.LongestStreak, summary.EndingStreak)
        summary.LastDay = key
    }

    l.Earlier = &summary
    return true
}
