package agent

const maxVerboseRune = 150

func truncateVerbose(s string) string {
	runes := []rune(s)
	if len(runes) <= maxVerboseRune {
		return s
	}
	return string(runes[:maxVerboseRune]) + "(... truncated)"
}
