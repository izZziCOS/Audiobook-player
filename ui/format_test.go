package ui

import "testing"

func TestFormatTime(t *testing.T) {
    for seconds, want := range map[float64]string{
        -5:    "0:00",
        0:     "0:00",
        59.9:  "0:59",
        61:    "1:01",
        3599:  "59:59",
        3600:  "1:00:00",
        18131: "5:02:11",
    } {
        if got := formatTime(seconds); got != want {
            t.Errorf("formatTime(%v) = %q, want %q", seconds, got, want)
        }
    }
}
