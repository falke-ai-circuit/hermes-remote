package features

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

type Task struct {
	Name        string
	Interval    time.Duration
	LastRun     time.Time
	NextRun     time.Time
	RunCount    int64
	LastElapsed time.Duration
	LastError   error
	taskFunc    func() error
}

type Scheduler struct {
	mu      sync.Mutex
	tasks   map[string]*Task
	stopCh  chan struct{}
	running bool
}

func NewScheduler() *Scheduler {
	return &Scheduler{tasks: make(map[string]*Task), stopCh: make(chan struct{})}
}

func (s *Scheduler) Register(name string, interval time.Duration, taskFunc func() error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.tasks[name]; exists {
		return fmt.Errorf("task already registered: %s", name)
	}
	s.tasks[name] = &Task{Name: name, Interval: interval, NextRun: time.Now().Add(interval), taskFunc: taskFunc}
	return nil
}

func (s *Scheduler) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running { return }
	s.running = true
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-s.stopCh: return
			case now := <-ticker.C: s.runDueTasks(now)
			}
		}
	}()
}

func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running { close(s.stopCh); s.running = false }
}

func (s *Scheduler) runDueTasks(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, task := range s.tasks {
		if now.Before(task.NextRun) { continue }
		start := time.Now()
		err := task.taskFunc()
		elapsed := time.Since(start)
		task.LastRun = start
		task.NextRun = now.Add(task.Interval)
		task.RunCount++
		task.LastElapsed = elapsed
		task.LastError = err
	}
}

type TaskInfo struct {
	Name        string    `json:"name"`
	Interval    string    `json:"interval"`
	LastRun     time.Time `json:"lastRun"`
	NextRun     time.Time `json:"nextRun"`
	RunCount    int64     `json:"runCount"`
	LastElapsed string    `json:"lastElapsed"`
	HasError    bool      `json:"hasError"`
}

func (s *Scheduler) GetTaskInfo() []TaskInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	infos := make([]TaskInfo, 0, len(s.tasks))
	for _, task := range s.tasks {
		infos = append(infos, TaskInfo{Name: task.Name, Interval: task.Interval.String(), LastRun: task.LastRun, NextRun: task.NextRun, RunCount: task.RunCount, LastElapsed: task.LastElapsed.String(), HasError: task.LastError != nil})
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })
	return infos
}

func (s *Scheduler) Unregister(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.tasks[name]; !exists { return fmt.Errorf("task not found: %s", name) }
	delete(s.tasks, name)
	return nil
}
