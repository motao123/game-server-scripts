package app

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestFileTaskManagerCancelsQueuedTask(t *testing.T) {
	m := &FileTaskManager{queue: make(chan fileTaskWork, 4)}
	var ranFirst atomic.Bool
	var ranSecond atomic.Bool
	first := m.Add("copy", "first", func(update func(int, string)) error {
		ranFirst.Store(true)
		time.Sleep(50 * time.Millisecond)
		return nil
	})
	second := m.Add("copy", "second", func(update func(int, string)) error {
		ranSecond.Store(true)
		return nil
	})
	if err := m.Cancel(second.ID); err != nil {
		t.Fatalf("cancel queued task: %v", err)
	}
	go m.run()
	deadline := time.Now().Add(time.Second)
	for !ranFirst.Load() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !ranFirst.Load() {
		t.Fatal("expected first task to run")
	}
	if ranSecond.Load() {
		t.Fatal("cancelled task must not run")
	}
	task, ok := m.Get(second.ID)
	if !ok || task.Status != "cancelled" || task.CanCancel {
		t.Fatalf("unexpected cancelled task: %#v", task)
	}
	if err := m.Cancel(first.ID); err == nil {
		t.Fatal("running or finished task should not be cancellable")
	}
}

func TestFileTaskManagerGetMissing(t *testing.T) {
	m := &FileTaskManager{queue: make(chan fileTaskWork, 1)}
	if _, ok := m.Get("missing"); ok {
		t.Fatal("missing task should not be found")
	}
	if err := m.Cancel("missing"); err == nil {
		t.Fatal("missing task cancel should fail")
	}
}
