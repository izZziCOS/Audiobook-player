package stats

import (
	"encoding/json"
	"math"
	"slices"
	"testing"
	"time"
)

func day(year int, month time.Month, date int) time.Time {
	return time.Date(year, month, date, 20, 0, 0, 0, time.Local)
}

func listen(log *Log, t time.Time, minutes float64, speed float64, book string) {
	for remaining := minutes * 60; remaining > 0; remaining -= 1 {
		log.Record(t, min(remaining, 1), speed, book)
	}
}

func TestRecordingAndTotals(t *testing.T) {
	log := New()
	monday := day(2026, time.October, 12)

	listen(log, monday, 30, 1.0, "dune")
	listen(log, monday.AddDate(0, 0, 1), 60, 1.5, "hobbit")
	log.Record(monday, 3600, 1, "dune")

	if total := log.Total(); math.Abs(total-(90*60+maxRecordedChunk)) > 1e-6 {
		t.Fatalf("total %v; a long jump between ticks should be capped", total)
	}

	speed := log.AverageSpeed()
	want := (30*60 + maxRecordedChunk + 60*60*1.5) / (90*60 + maxRecordedChunk)
	if math.Abs(speed-want) > 1e-9 {
		t.Fatalf("average speed %v, want %v", speed, want)
	}

	top := log.TopBooks(5)
	if len(top) != 2 || top[0].Key != "hobbit" || top[1].Key != "dune" {
		t.Fatalf("top books %+v", top)
	}

	log.Record(monday, 0, 1, "dune")
	log.Record(monday, -5, 1, "dune")
	if len(log.TopBooks(5)) != 2 {
		t.Fatal("nothing should be recorded for zero or negative time")
	}
}

func TestDailyAndWeekly(t *testing.T) {
	log := New()
	wednesday := day(2026, time.October, 14)

	listen(log, day(2026, time.October, 12), 10, 1, "a")
	listen(log, wednesday, 20, 1, "a")
	listen(log, day(2026, time.October, 5), 40, 1, "a")

	daily := log.Daily(wednesday, 3)
	if len(daily) != 3 || daily[0].Start.Day() != 12 || daily[2].Start.Day() != 14 {
		t.Fatalf("daily periods %+v", daily)
	}
	if daily[0].Seconds != 600 || daily[1].Seconds != 0 || daily[2].Seconds != 1200 {
		t.Fatalf("daily seconds %v %v %v", daily[0].Seconds, daily[1].Seconds, daily[2].Seconds)
	}

	weekly := log.Weekly(wednesday, 2)
	if weekly[1].Start.Weekday() != time.Monday || weekly[1].Start.Day() != 12 {
		t.Fatalf("weeks should start on Monday, got %v", weekly[1].Start)
	}
	if weekly[0].Seconds != 2400 || weekly[1].Seconds != 1800 {
		t.Fatalf("weekly seconds %v %v", weekly[0].Seconds, weekly[1].Seconds)
	}

	if log.ThisWeek(wednesday) != 1800 {
		t.Fatalf("this week %v", log.ThisWeek(wednesday))
	}
	if average := log.DailyAverage(wednesday, 3); average != 600 {
		t.Fatalf("daily average %v", average)
	}

	sunday := log.Weekly(day(2026, time.October, 18), 1)[0]
	if sunday.Start.Day() != 12 {
		t.Fatalf("Sunday belongs to the week starting Monday 12th, got %v", sunday.Start)
	}
}

func TestStreaks(t *testing.T) {
	log := New()
	today := day(2026, time.October, 20)

	for _, date := range []int{1, 2, 3, 4, 5, 10, 17, 18, 19} {
		listen(log, day(2026, time.October, date), 5, 1, "a")
	}
	listen(log, day(2026, time.October, 11), 0.5, 1, "a")

	current, longest := log.Streaks(today)
	if current != 3 || longest != 5 {
		t.Fatalf("before listening today: current %d, longest %d", current, longest)
	}

	listen(log, today, 2, 1, "a")
	if current, _ := log.Streaks(today); current != 4 {
		t.Fatalf("listening today should extend the streak, got %d", current)
	}

	if current, _ := log.Streaks(day(2026, time.October, 22)); current != 0 {
		t.Fatalf("missing a day should end the streak, got %d", current)
	}

	if current, longest := New().Streaks(today); current != 0 || longest != 0 {
		t.Fatal("an empty log has no streaks")
	}
}

func TestCompletedAndSaving(t *testing.T) {
	log := New()
	log.Complete(day(2026, time.October, 1), "dune")
	log.Complete(day(2026, time.October, 9), "dune")
	log.Complete(day(2026, time.October, 9), "hobbit")
	listen(log, day(2026, time.October, 9), 1, 1.25, "dune")

	if log.CompletedCount() != 2 {
		t.Fatalf("finishing a book twice counts once, got %d", log.CompletedCount())
	}

	encoded, err := json.Marshal(log)
	if err != nil {
		t.Fatal(err)
	}

	var loaded Log
	if err := json.Unmarshal(encoded, &loaded); err != nil {
		t.Fatal(err)
	}
	if loaded.CompletedCount() != 2 || loaded.Total() != 60 || loaded.AverageSpeed() != 1.25 {
		t.Fatalf("reloaded log %+v", loaded)
	}

	var empty Log
	empty.Record(day(2026, time.October, 9), 1, 1, "a")
	if empty.Total() != 1 {
		t.Fatal("a zero-value log should still record")
	}
}

func TestRekeyMovesABooksHistory(t *testing.T) {
	log := New()
	listen(log, day(2026, time.October, 9), 2, 1, "E:/Books/Dune.m4b")
	log.Complete(day(2026, time.October, 9), "E:/Books/Dune.m4b")
	listen(log, day(2026, time.October, 10), 1, 1, "F:/Books/Dune.m4b")

	log.Rekey("E:/Books/Dune.m4b", "F:/Books/Dune.m4b")

	if log.Books["F:/Books/Dune.m4b"] != 180 || len(log.Books) != 1 {
		t.Fatalf("books %v", log.Books)
	}
	if log.Completed["F:/Books/Dune.m4b"] != "2026-10-09" || log.CompletedCount() != 1 {
		t.Fatalf("completed %v", log.Completed)
	}

	log.Rekey("missing", "anywhere")
	if len(log.Books) != 1 || len(log.Completed) != 1 {
		t.Fatal("re-keying an unknown book should change nothing")
	}
}

func randomHistory(seed uint32, today time.Time, days int) *Log {
	random := func() float64 {
		seed = seed*1664525 + 1013904223
		return float64(seed>>8) / float64(1<<24)
	}

	log := New()
	for daysAgo := range days {
		switch roll := random(); {
		case roll < 0.25:
			continue
		case roll < 0.35:
			log.Days[dayKey(today.AddDate(0, 0, -daysAgo))] = Day{Listened: 30, Content: 30}
		default:
			minutes := 1 + random()*90
			log.Days[dayKey(today.AddDate(0, 0, -daysAgo))] = Day{Listened: minutes * 60, Content: minutes * 60 * (1 + random())}
		}
	}

	return log
}

func copyLog(log *Log) *Log {
	data, _ := json.Marshal(log)
	var copied Log
	json.Unmarshal(data, &copied)
	return &copied
}

func TestCompactingKeepsEveryStatistic(t *testing.T) {
	today := time.Date(2026, 10, 20, 15, 0, 0, 0, time.Local)

	for seed := uint32(1); seed <= 40; seed++ {
		for _, keep := range []int{1, 2, 7, 30, 366} {
			full := randomHistory(seed, today, 900)
			compacted := copyLog(full)

			for daysLater := 0; daysLater <= 3; daysLater++ {
				compacted.Compact(today.AddDate(0, 0, -3+daysLater), keep)
			}
			compacted = copyLog(compacted)

			if len(compacted.Days) > keep {
				t.Fatalf("seed %d keep %d: %d days left", seed, keep, len(compacted.Days))
			}
			if math.Abs(full.Total()-compacted.Total()) > 1e-6 || math.Abs(full.AverageSpeed()-compacted.AverageSpeed()) > 1e-9 {
				t.Fatalf("seed %d keep %d: total %v / %v, speed %v / %v", seed, keep, full.Total(), compacted.Total(), full.AverageSpeed(), compacted.AverageSpeed())
			}

			fullCurrent, fullLongest := full.Streaks(today)
			current, longest := compacted.Streaks(today)
			if current != fullCurrent || longest != fullLongest {
				t.Fatalf("seed %d keep %d: streaks %d/%d, want %d/%d", seed, keep, current, longest, fullCurrent, fullLongest)
			}

			for _, check := range []struct {
				name       string
				full, kept []Period
			}{
				{"daily", full.Daily(today, min(keep, 30)), compacted.Daily(today, min(keep, 30))},
				{"weekly", full.Weekly(today, keep/7), compacted.Weekly(today, keep/7)},
			} {
				if !slices.Equal(check.full, check.kept) {
					t.Fatalf("seed %d keep %d: the %s chart changed", seed, keep, check.name)
				}
			}
		}
	}
}

func TestLongStreaksSurviveCompacting(t *testing.T) {
	today := time.Date(2026, 10, 20, 9, 0, 0, 0, time.Local)
	log := New()
	for daysAgo := range 500 {
		log.Days[dayKey(today.AddDate(0, 0, -daysAgo))] = Day{Listened: 3600, Content: 3600}
	}

	if !log.Compact(today, KeepDays) || log.Compact(today, KeepDays) {
		t.Fatal("compacting should change the log once, and then have nothing left to do")
	}
	if current, longest := log.Streaks(today); current != 500 || longest != 500 {
		t.Fatalf("a 500-day streak became %d, longest %d", current, longest)
	}
	if len(log.Days) != KeepDays {
		t.Fatalf("%d days kept, want %d", len(log.Days), KeepDays)
	}
}

func TestCombiningComputers(t *testing.T) {
	now := time.Date(2026, 10, 21, 20, 0, 0, 0, time.Local)
	laptop, desktop := New(), New()
	today, yesterday := dayKey(now), dayKey(now.AddDate(0, 0, -1))
	laptop.Days[today] = Day{Listened: 40, Content: 60}
	laptop.Books["/books/Dune"] = 40
	desktop.Days[today] = Day{Listened: 40, Content: 40}
	desktop.Books["/books/Dune"] = 40
	desktop.Days[yesterday] = Day{Listened: 120, Content: 120}
	desktop.Books["/books/Emma"] = 120
	laptop.Complete(now.AddDate(0, 0, -3), "/books/Emma")
	desktop.Complete(now, "/books/Emma")
	laptop.Earlier = &Summary{Listened: 100, Content: 100, LongestStreak: 4, EndingStreak: 2, LastDay: "2025-01-01"}
	desktop.Earlier = &Summary{Listened: 50, Content: 50, LongestStreak: 6, EndingStreak: 1, LastDay: "2025-02-01"}

	both := Combine(laptop, desktop, nil)

	if both.Listened(now) != 80 || both.Books["/books/Dune"] != 80 || both.Total() != 80+120+150 {
		t.Fatalf("listening should add up: today %v, Dune %v, total %v", both.Listened(now), both.Books["/books/Dune"], both.Total())
	}
	if both.CompletedCount() != 1 || both.Completed["/books/Emma"] != "2026-10-21" {
		t.Fatalf("a book finished on two computers counts once, got %v", both.Completed)
	}
	if current, _ := both.Streaks(now); current != 2 {
		t.Fatalf("a minute between both computers today keeps the streak going, got %d", current)
	}
	if both.Earlier.LongestStreak != 6 || both.Earlier.LastDay != "2025-02-01" || both.Earlier.EndingStreak != 1 {
		t.Fatalf("older summaries combine too, got %+v", both.Earlier)
	}
	if laptop.Books["/books/Dune"] != 40 {
		t.Fatal("combining must not change the computers' own logs")
	}
}
