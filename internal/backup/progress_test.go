package backup

import (
	"testing"
	"time"
)

func TestProgressEmitterThrottlesRapidRowsAndKeepsBoundaries(t *testing.T) {
	now := time.Unix(0, 0)
	var emitted []Progress
	emitter := newProgressEmitter(func(progress Progress) {
		emitted = append(emitted, progress)
	}, 250*time.Millisecond, func() time.Time { return now })

	emitter.Emit(Progress{Current: 0, Total: 100, Message: "开始备份"})
	for current := 1; current < 100; current++ {
		now = now.Add(time.Millisecond)
		emitter.Emit(Progress{Current: current, Total: 100})
	}
	now = now.Add(time.Millisecond)
	emitter.Emit(Progress{Current: 100, Total: 100})

	if len(emitted) != 2 {
		t.Fatalf("emitted %d progress events, want initial and final only", len(emitted))
	}
	if emitted[0].Current != 0 || emitted[1].Current != 100 {
		t.Fatalf("emitted boundaries = [%d, %d], want [0, 100]", emitted[0].Current, emitted[1].Current)
	}
}

func TestProgressEmitterPublishesAtInteractiveCadence(t *testing.T) {
	now := time.Unix(0, 0)
	var currents []int
	emitter := newProgressEmitter(func(progress Progress) {
		currents = append(currents, progress.Current)
	}, 250*time.Millisecond, func() time.Time { return now })

	emitter.Emit(Progress{Current: 0, Total: 100})
	now = now.Add(249 * time.Millisecond)
	emitter.Emit(Progress{Current: 25, Total: 100})
	now = now.Add(time.Millisecond)
	emitter.Emit(Progress{Current: 26, Total: 100})

	if len(currents) != 2 || currents[0] != 0 || currents[1] != 26 {
		t.Fatalf("emitted currents = %v, want [0 26]", currents)
	}
}
