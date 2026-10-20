package kit

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
        if got := FormatTime(seconds); got != want {
            t.Errorf("FormatTime(%v) = %q, want %q", seconds, got, want)
        }
    }
}

func TestFormatLength(t *testing.T) {
    for seconds, want := range map[float64]string{
        0:     "0m",
        5:     "1m",
        125:   "2m",
        3599:  "1h 0m",
        18720: "5h 12m",
    } {
        if got := FormatLength(seconds); got != want {
            t.Errorf("FormatLength(%v) = %q, want %q", seconds, got, want)
        }
    }
}
