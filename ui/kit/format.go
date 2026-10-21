package kit

import (
	"fmt"
	"math"
)

func FormatLength(seconds float64) string {
	minutes := int(math.Round(seconds / 60))
	if seconds > 0 {
		minutes = max(minutes, 1)
	}

	if minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}

	return fmt.Sprintf("%dh %dm", minutes/60, minutes%60)
}

func FormatListening(seconds float64) string {
	if seconds > 0 && seconds < 60 {
		return "Under 1m"
	}

	return FormatLength(seconds)
}

func FormatTime(seconds float64) string {
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

func CountPhrase(count int, singular, plural, none string) string {
	switch count {
	case 0:
		return none
	case 1:
		return "1 " + singular
	default:
		return fmt.Sprintf("%d %s", count, plural)
	}
}
