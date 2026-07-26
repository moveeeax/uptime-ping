package monitor

// State is the health state a service is reported in.
type State string

const (
	// StateUnknown is the initial state before enough samples exist.
	StateUnknown State = "unknown"
	// StateUp means the service is healthy.
	StateUp State = "up"
	// StateDown means the service is failing.
	StateDown State = "down"
)

// stateMachine turns a stream of pass/fail observations into a debounced
// up/down state using consecutive-count thresholds. It contains no I/O and
// is fully deterministic, which makes the transition logic unit-testable.
type stateMachine struct {
	state       State
	okStreak    int
	failStreak  int
	failNeeded  int
	okNeeded    int
	initialized bool
}

func newStateMachine(failThreshold, successThreshold int) *stateMachine {
	if failThreshold < 1 {
		failThreshold = 1
	}
	if successThreshold < 1 {
		successThreshold = 1
	}
	return &stateMachine{
		state:      StateUnknown,
		failNeeded: failThreshold,
		okNeeded:   successThreshold,
	}
}

// observe records one cycle result and reports whether the reported state
// changed as a consequence.
func (s *stateMachine) observe(ok bool) (changed bool) {
	if ok {
		s.okStreak++
		s.failStreak = 0
	} else {
		s.failStreak++
		s.okStreak = 0
	}

	var next State
	switch {
	case ok && s.okStreak >= s.okNeeded:
		next = StateUp
	case !ok && s.failStreak >= s.failNeeded:
		next = StateDown
	default:
		next = s.state
	}

	if !s.initialized {
		// The first decisive observation seeds the state without counting
		// as a transition (nothing to alert about on startup).
		if next != StateUnknown {
			s.initialized = true
			s.state = next
		}
		return false
	}

	if next != s.state {
		s.state = next
		return true
	}
	return false
}
