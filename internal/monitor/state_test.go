package monitor

import "testing"

func TestStateMachineSimpleFlip(t *testing.T) {
	sm := newStateMachine(1, 1)

	if changed := sm.observe(true); changed {
		t.Error("first up observation should seed state, not report a change")
	}
	if sm.state != StateUp {
		t.Fatalf("state = %s, want up", sm.state)
	}
	if changed := sm.observe(false); !changed {
		t.Error("up->down should report a change")
	}
	if sm.state != StateDown {
		t.Fatalf("state = %s, want down", sm.state)
	}
	if changed := sm.observe(false); changed {
		t.Error("down->down should not report a change")
	}
}

func TestStateMachineFailureThreshold(t *testing.T) {
	// Require 3 consecutive failures before flipping to down.
	sm := newStateMachine(3, 1)
	sm.observe(true) // seed up

	if sm.observe(false) || sm.observe(false) {
		t.Fatal("should not flip before threshold reached")
	}
	if sm.state != StateUp {
		t.Fatalf("premature flip, state = %s", sm.state)
	}
	if !sm.observe(false) {
		t.Fatal("third failure should flip to down")
	}
	if sm.state != StateDown {
		t.Fatalf("state = %s, want down", sm.state)
	}
}

func TestStateMachineSuccessThreshold(t *testing.T) {
	sm := newStateMachine(1, 2)
	sm.observe(false) // seed down

	if sm.observe(true) {
		t.Fatal("single success should not flip when threshold is 2")
	}
	if !sm.observe(true) {
		t.Fatal("second success should flip to up")
	}
	if sm.state != StateUp {
		t.Fatalf("state = %s, want up", sm.state)
	}
}

func TestStateMachineFlappingResetsStreak(t *testing.T) {
	sm := newStateMachine(3, 1)
	sm.observe(true) // seed up
	sm.observe(false)
	sm.observe(false)
	sm.observe(true) // resets the failure streak
	sm.observe(false)
	sm.observe(false)
	if sm.state != StateUp {
		t.Fatalf("flapping should not have flipped to down, state = %s", sm.state)
	}
}
