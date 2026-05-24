package scheduler

import (
	"container/heap"
	"time"
)

type JobStatus int

const (
	Queued JobStatus = iota
	Running
	Done
	Failed
)

func (s JobStatus) String() string {
	switch s {
	case Queued:
		return "Queued"
	case Running:
		return "Running"
	case Done:
		return "Done"
	case Failed:
		return "Failed"
	default:
		return "Unknown"
	}
}

type Job struct {
	ID          string
	Command     string
	NumGPUs     int
	MinVRAMMB   uint64
	Priority    int
	Status      JobStatus
	SubmittedAt time.Time
	StartedAt   time.Time
	GPUIDs      []int
	PID         int
}

type JobQueue []*Job

func (q JobQueue) Len() int { return len(q) }

func (q JobQueue) Less(i, j int) bool {
	if q[i].Priority != q[j].Priority {
		return q[i].Priority < q[j].Priority
	}
	return q[i].SubmittedAt.Before(q[j].SubmittedAt)
}

func (q JobQueue) Swap(i, j int) {
	q[i], q[j] = q[j], q[i]
}

func (q *JobQueue) Push(x interface{}) {
	*q = append(*q, x.(*Job))
}

func (q *JobQueue) Pop() interface{} {
	old := *q
	n := len(old)
	if n == 0 {
		return nil
	}
	job := old[n-1]
	old[n-1] = nil
	*q = old[:n-1]
	return job
}

func (q *JobQueue) PushJob(job *Job) {
	heap.Push(q, job)
}

func (q *JobQueue) PopJob() *Job {
	if q.Len() == 0 {
		return nil
	}
	return heap.Pop(q).(*Job)
}
