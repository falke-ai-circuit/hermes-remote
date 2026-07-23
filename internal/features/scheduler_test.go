package features

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestScheduler_Register verifies that a task is registered and can be retrieved.
func TestScheduler_Register(t *testing.T) {
	s := NewScheduler()
	defer s.Stop()

	called := atomic.Int64{}
	err := s.Register("test-task", 1*time.Hour, func() error {
		called.Add(1)
		return nil
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	infos := s.GetTaskInfo()
	if len(infos) != 1 {
		t.Fatalf("expected 1 task, got %d", len(infos))
	}
	if infos[0].Name != "test-task" {
		t.Errorf("Name: got %q, want %q", infos[0].Name, "test-task")
	}
}

// TestScheduler_Register_Duplicate verifies that registering a task
// with the same name returns an error.
func TestScheduler_Register_Duplicate(t *testing.T) {
	s := NewScheduler()
	defer s.Stop()

	err := s.Register("task1", 1*time.Hour, func() error { return nil })
	if err != nil {
		t.Fatalf("first Register: %v", err)
	}

	err = s.Register("task1", 1*time.Hour, func() error { return nil })
	if err == nil {
		t.Fatal("expected error for duplicate task name")
	}
}

// TestScheduler_Unregister verifies that a task is removed.
func TestScheduler_Unregister(t *testing.T) {
	s := NewScheduler()
	defer s.Stop()

	_ = s.Register("task1", 1*time.Hour, func() error { return nil })

	err := s.Unregister("task1")
	if err != nil {
		t.Fatalf("Unregister: %v", err)
	}

	infos := s.GetTaskInfo()
	if len(infos) != 0 {
		t.Fatalf("expected 0 tasks after unregister, got %d", len(infos))
	}
}

// TestScheduler_Unregister_NotFound verifies that unregistering a
// non-existent task returns an error.
func TestScheduler_Unregister_NotFound(t *testing.T) {
	s := NewScheduler()
	defer s.Stop()

	err := s.Unregister("no-such-task")
	if err == nil {
		t.Error("expected error for non-existent task")
	}
}

// TestScheduler_RunDueTasks verifies that due tasks are executed.
func TestScheduler_RunDueTasks(t *testing.T) {
	s := NewScheduler()
	defer s.Stop()

	called := atomic.Int64{}
	err := s.Register("task1", 1*time.Millisecond, func() error {
		called.Add(1)
		return nil
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	// Wait for NextRun to be in the past
	time.Sleep(5 * time.Millisecond)

	// Run due tasks manually
	s.runDueTasks(time.Now())

	if called.Load() != 1 {
		t.Errorf("expected 1 call, got %d", called.Load())
	}
}

// TestScheduler_RunDueTasks_NotDue verifies that tasks that are not
// yet due are not executed.
func TestScheduler_RunDueTasks_NotDue(t *testing.T) {
	s := NewScheduler()
	defer s.Stop()

	called := atomic.Int64{}
	err := s.Register("task1", 1*time.Hour, func() error {
		called.Add(1)
		return nil
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	// Task is scheduled 1 hour in the future — not due yet
	s.runDueTasks(time.Now())

	if called.Load() != 0 {
		t.Errorf("expected 0 calls (not due), got %d", called.Load())
	}
}

// TestScheduler_StartStop verifies that Start launches the background
// loop and Stop terminates it.
func TestScheduler_StartStop(t *testing.T) {
	s := NewScheduler()

	called := atomic.Int64{}
	err := s.Register("task1", 10*time.Millisecond, func() error {
		called.Add(1)
		return nil
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	s.Start()

	// The scheduler ticks every 1 second, so we need to wait > 1s
	time.Sleep(2 * time.Second)

	s.Stop()

	if called.Load() == 0 {
		t.Error("expected task to run at least once after Start")
	}
}

// TestScheduler_Stop_Idempotent verifies that Stop is safe to call
// multiple times.
func TestScheduler_Stop_Idempotent(t *testing.T) {
	s := NewScheduler()
	s.Start()
	s.Stop()
	s.Stop()
	s.Stop()
	// Should not panic
}

// TestScheduler_Start_Idempotent verifies that calling Start multiple
// times does not launch multiple goroutines.
func TestScheduler_Start_Idempotent(t *testing.T) {
	s := NewScheduler()
	defer s.Stop()

	s.Start()
	s.Start()
	s.Start()
	// Should only have one running goroutine
	s.mu.Lock()
	running := s.running
	s.mu.Unlock()
	if !running {
		t.Error("expected running=true after Start")
	}
}

// TestScheduler_TaskError verifies that task errors are recorded.
func TestScheduler_TaskError(t *testing.T) {
	s := NewScheduler()
	defer s.Stop()

	errMsg := fmt.Errorf("task failed")
	_ = s.Register("task1", 1*time.Millisecond, func() error {
		return errMsg
	})

	time.Sleep(5 * time.Millisecond)
	s.runDueTasks(time.Now())

	infos := s.GetTaskInfo()
	if len(infos) != 1 {
		t.Fatalf("expected 1 task, got %d", len(infos))
	}
	if !infos[0].HasError {
		t.Error("expected task to have error")
	}
}

// TestScheduler_RunCount verifies that RunCount is incremented after
// each execution.
func TestScheduler_RunCount(t *testing.T) {
	s := NewScheduler()
	defer s.Stop()

	_ = s.Register("task1", 1*time.Millisecond, func() error { return nil })

	// Run multiple times
	for i := 0; i < 3; i++ {
		time.Sleep(2 * time.Millisecond)
		s.runDueTasks(time.Now())
	}

	infos := s.GetTaskInfo()
	if infos[0].RunCount < 1 {
		t.Errorf("expected RunCount >= 1, got %d", infos[0].RunCount)
	}
}

// TestScheduler_GetTaskInfo_Sorted verifies that GetTaskInfo returns
// tasks sorted by name.
func TestScheduler_GetTaskInfo_Sorted(t *testing.T) {
	s := NewScheduler()
	defer s.Stop()

	_ = s.Register("zebra", 1*time.Hour, func() error { return nil })
	_ = s.Register("apple", 1*time.Hour, func() error { return nil })
	_ = s.Register("mango", 1*time.Hour, func() error { return nil })

	infos := s.GetTaskInfo()
	if len(infos) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(infos))
	}
	if infos[0].Name != "apple" {
		t.Errorf("expected first task 'apple', got %q", infos[0].Name)
	}
	if infos[1].Name != "mango" {
		t.Errorf("expected second task 'mango', got %q", infos[1].Name)
	}
	if infos[2].Name != "zebra" {
		t.Errorf("expected third task 'zebra', got %q", infos[2].Name)
	}
}

// TestScheduler_GetTaskInfo_Empty verifies that an empty scheduler
// returns an empty slice.
func TestScheduler_GetTaskInfo_Empty(t *testing.T) {
	s := NewScheduler()
	defer s.Stop()
	infos := s.GetTaskInfo()
	if len(infos) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(infos))
	}
}

// TestScheduler_ConcurrentAccess verifies that concurrent operations
// on the scheduler don't cause data races. Run with -race.
func TestScheduler_ConcurrentAccess(t *testing.T) {
	s := NewScheduler()
	defer s.Stop()

	_ = s.Register("task1", 1*time.Millisecond, func() error { return nil })
	_ = s.Register("task2", 1*time.Millisecond, func() error { return nil })

	var wg sync.WaitGroup

	// Concurrent readers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				s.GetTaskInfo()
			}
		}()
	}

	// Concurrent runner
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 20; j++ {
			s.runDueTasks(time.Now())
		}
	}()

	// Concurrent register/unregister
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 20; j++ {
			_ = s.Register(fmt.Sprintf("dynamic-%d", j), 1*time.Hour, func() error { return nil })
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 20; j++ {
			_ = s.Unregister(fmt.Sprintf("dynamic-%d", j))
		}
	}()

	wg.Wait()
}

// TestScheduler_RunDueTasks_MultipleTasks verifies that multiple due
// tasks are all executed.
func TestScheduler_RunDueTasks_MultipleTasks(t *testing.T) {
	s := NewScheduler()
	defer s.Stop()

	called1 := atomic.Int64{}
	called2 := atomic.Int64{}
	called3 := atomic.Int64{}

	_ = s.Register("task1", 1*time.Millisecond, func() error {
		called1.Add(1)
		return nil
	})
	_ = s.Register("task2", 1*time.Millisecond, func() error {
		called2.Add(1)
		return nil
	})
	_ = s.Register("task3", 1*time.Millisecond, func() error {
		called3.Add(1)
		return nil
	})

	time.Sleep(5 * time.Millisecond)
	s.runDueTasks(time.Now())

	if called1.Load() != 1 {
		t.Errorf("task1: expected 1 call, got %d", called1.Load())
	}
	if called2.Load() != 1 {
		t.Errorf("task2: expected 1 call, got %d", called2.Load())
	}
	if called3.Load() != 1 {
		t.Errorf("task3: expected 1 call, got %d", called3.Load())
	}
}

// TestScheduler_Start_RunsRepeatedly verifies that the scheduler
// background loop executes tasks multiple times.
func TestScheduler_Start_RunsRepeatedly(t *testing.T) {
	s := NewScheduler()

	called := atomic.Int64{}
	_ = s.Register("task1", 5*time.Millisecond, func() error {
		called.Add(1)
		return nil
	})

	s.Start()
	// The scheduler ticks every 1 second; wait long enough for multiple runs
	time.Sleep(3 * time.Second)
	s.Stop()

	if called.Load() < 2 {
		t.Errorf("expected task to run at least 2 times, got %d", called.Load())
	}
}