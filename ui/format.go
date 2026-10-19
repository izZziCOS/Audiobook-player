package ui

import "fmt"

func formatTime(seconds float64) string {
    if seconds < 0 {
        seconds = 0
    }

    totalSeconds := int(seconds)
    hours := totalSeconds / 3600
    minutes := (totalSeconds % 3600) / 60
    remainingSeconds := totalSeconds % 60

    if hours > 0 {
        return fmt.Sprintf(
            "%d:%02d:%02d",
            hours,
            minutes,
            remainingSeconds,
        )
    }

    return fmt.Sprintf(
        "%d:%02d",
        minutes,
        remainingSeconds,
    )
}
