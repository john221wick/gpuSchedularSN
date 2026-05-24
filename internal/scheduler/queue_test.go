package scheduler

import (
	"testing"
	"time"
)

func TestQueuePriorityOrder(t *testing.T) {
	q := &JobQueue{}
	now := time.Now()

	q.PushJob(&Job{ID: "a", Priority: 5, SubmittedAt: now})
	q.PushJob(&Job{ID: "b", Priority: 1, SubmittedAt: now})
	q.PushJob(&Job{ID: "c", Priority: 3, SubmittedAt: now})

	first := q.PopJob()
	second := q.PopJob()
	third := q.PopJob()

	if first.ID != "b" {
		t.Errorf("first pop = %s, want b (priority 1)", first.ID)
	}
	if second.ID != "c" {
		t.Errorf("second pop = %s, want c (priority 3)", second.ID)
	}
	if third.ID != "a" {
		t.Errorf("third pop = %s, want a (priority 5)", third.ID)
	}
}

func TestQueueFIFOForSamePriority(t *testing.T) {
	q := &JobQueue{}
	base := time.Now()

	q.PushJob(&Job{ID: "first", Priority: 1, SubmittedAt: base})
	q.PushJob(&Job{ID: "second", Priority: 1, SubmittedAt: base.Add(time.Second)})
	q.PushJob(&Job{ID: "third", Priority: 1, SubmittedAt: base.Add(2 * time.Second)})

	if q.PopJob().ID != "first" {
		t.Error("expected first job")
	}
	if q.PopJob().ID != "second" {
		t.Error("expected second job")
	}
	if q.PopJob().ID != "third" {
		t.Error("expected third job")
	}
}

func TestQueuePushPopSingle(t *testing.T) {
	q := &JobQueue{}

	q.PushJob(&Job{ID: "only", Priority: 1, SubmittedAt: time.Now()})

	job := q.PopJob()
	if job == nil {
		t.Fatal("PopJob returned nil")
	}
	if job.ID != "only" {
		t.Errorf("got %s, want only", job.ID)
	}
}

func TestQueueEmptyPop(t *testing.T) {
	q := &JobQueue{}

	job := q.PopJob()
	if job != nil {
		t.Errorf("expected nil from empty queue, got %v", job)
	}
}
