package domain

// StatusRank returns progression rank. Higher is further along (except rejected).
func StatusRank(status string) int {
	switch status {
	case StatusApplied:
		return 1
	case StatusAssessment:
		return 2
	case StatusInterview:
		return 3
	case StatusAccepted:
		return 4
	case StatusRejected:
		return 0 // handled specially
	default:
		return 0
	}
}

// ShouldUpdateStatus reports whether next should replace current.
// Forward progression only, except rejected may override anything except accepted.
func ShouldUpdateStatus(current, next string) bool {
	if next == "" || next == current {
		return false
	}
	if next == StatusRejected {
		return current != StatusAccepted
	}
	if current == StatusRejected {
		// Allow moving out of rejected only to accepted (rare).
		return next == StatusAccepted
	}
	return StatusRank(next) > StatusRank(current)
}
