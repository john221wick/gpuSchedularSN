package agentserver

import (
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/john221wick/gpuSchedularSN/internal/agent"
)

// Sampler measures CPU continuously by diffing kernel counters between
// ticks. /monitor serves the latest sample, so the request path never
// sleeps or forks, and no activity between requests is missed.
const (
	sampleInterval  = 2 * time.Second
	heavyEvery      = 10 * time.Second // docker + nvidia-smi cadence
	historyCapacity = 300              // ~10 min of points at 2s ticks
	maxProcs        = 120
)

// HistoryPoint is one entry of the rolling host-stats timeline.
type HistoryPoint struct {
	TS         string  `json:"ts"`
	CPUPercent float64 `json:"cpuPercent"`
	MemUsedMB  uint64  `json:"memUsedMB"`
	MemTotalMB uint64  `json:"memTotalMB"`
}

// procStat is the per-tick reading for one process.
type procStat struct {
	comm  string
	ticks uint64 // utime+stime in USER_HZ clock ticks
	rssMB float64
}

type Sampler struct {
	mu      sync.RWMutex
	latest  MonitorResponse
	history []HistoryPoint

	stopOnce sync.Once
	stopCh   chan struct{}

	// Previous-tick counter state; only the run loop touches these.
	prevAgg   cpuTimes
	prevCores []cpuTimes
	prevProcs map[int]uint64

	// Expensive externals (docker, nvidia-smi), refreshed on heavyEvery.
	containers ContainerReport
	gpus       []agent.GPUDevice
	gpuProcs   []GPUProcInfo
	lastHeavy  time.Time
}

func NewSampler() *Sampler {
	return &Sampler{
		stopCh:    make(chan struct{}),
		prevProcs: map[int]uint64{},
	}
}

// Start primes the counters and publishes an immediate static snapshot
// (CPU fields zero), then keeps sampling in the background. The first
// accurate CPU sample lands ~150ms later; after that, every tick.
func (s *Sampler) Start() {
	s.prevAgg, s.prevCores = readCPUTimes()
	for pid, st := range readProcStats() {
		s.prevProcs[pid] = st.ticks
	}
	s.refreshHeavy()

	hs := collectHostStats()
	s.publish(MonitorResponse{
		Host:         hs,
		GPUs:         s.gpus,
		Containers:   s.containers,
		Processes:    []ProcInfo{},
		GPUProcesses: s.gpuProcs,
		CollectedAt:  time.Now().Format(time.RFC3339),
	})

	go func() {
		select {
		case <-time.After(150 * time.Millisecond):
			s.tick()
		case <-s.stopCh:
			return
		}
		ticker := time.NewTicker(sampleInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.tick()
			case <-s.stopCh:
				return
			}
		}
	}()
}

func (s *Sampler) Stop() {
	s.stopOnce.Do(func() { close(s.stopCh) })
}

func (s *Sampler) Latest() MonitorResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.latest
}

func (s *Sampler) History() []HistoryPoint {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]HistoryPoint, len(s.history))
	copy(out, s.history)
	return out
}

func (s *Sampler) refreshHeavy() {
	s.containers = collectContainers()
	s.gpus = agent.Refresh()
	s.gpuProcs = collectGPUProcesses()
	s.lastHeavy = time.Now()
}

func (s *Sampler) tick() {
	hs := collectHostStats()
	procs := []ProcInfo{}

	if runtime.GOOS == "linux" {
		agg, cores := readCPUTimes()
		hs.CPUPercent = busyPct(s.prevAgg, agg)
		n := min(len(cores), len(s.prevCores))
		hs.PerCoreCPU = make([]float64, n)
		for i := 0; i < n; i++ {
			hs.PerCoreCPU[i] = busyPct(s.prevCores[i], cores[i])
		}

		// Per-process %: delta of the process's CPU ticks over the
		// wall-clock ticks one core had this window (top's convention:
		// 100% = one full core, multithreaded can exceed 100).
		cur := readProcStats()
		ncores := len(cores)
		if ncores == 0 {
			ncores = runtime.NumCPU()
		}
		wallPerCore := float64(agg.total-s.prevAgg.total) / float64(ncores)
		nextTicks := make(map[int]uint64, len(cur))
		for pid, st := range cur {
			nextTicks[pid] = st.ticks
			var pct float64
			// A missing or larger previous reading means a new (or
			// reused) PID; report 0 until it has a full window.
			if prev, ok := s.prevProcs[pid]; ok && st.ticks >= prev && wallPerCore > 0 {
				pct = float64(st.ticks-prev) / wallPerCore * 100
			}
			procs = append(procs, ProcInfo{
				PID:        pid,
				Command:    st.comm,
				CPUPercent: pct,
				MemMB:      st.rssMB,
			})
		}
		sort.Slice(procs, func(i, j int) bool {
			if procs[i].CPUPercent != procs[j].CPUPercent {
				return procs[i].CPUPercent > procs[j].CPUPercent
			}
			return procs[i].MemMB > procs[j].MemMB
		})
		if len(procs) > maxProcs {
			procs = procs[:maxProcs]
		}
		s.prevAgg, s.prevCores, s.prevProcs = agg, cores, nextTicks
	}

	if time.Since(s.lastHeavy) >= heavyEvery {
		s.refreshHeavy()
	}

	s.publish(MonitorResponse{
		Host:         hs,
		GPUs:         s.gpus,
		Containers:   s.containers,
		Processes:    procs,
		GPUProcesses: s.gpuProcs,
		CollectedAt:  time.Now().Format(time.RFC3339),
	})
}

func (s *Sampler) publish(mon MonitorResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.latest = mon
	s.history = append(s.history, HistoryPoint{
		TS:         mon.CollectedAt,
		CPUPercent: mon.Host.CPUPercent,
		MemUsedMB:  mon.Host.MemUsedMB,
		MemTotalMB: mon.Host.MemTotalMB,
	})
	if len(s.history) > historyCapacity {
		s.history = s.history[len(s.history)-historyCapacity:]
	}
}

// readProcStats reads utime+stime and RSS for every PID in /proc.
func readProcStats() map[int]procStat {
	res := map[int]procStat{}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return res
	}
	pageMB := float64(os.Getpagesize()) / (1024 * 1024)
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		b, err := os.ReadFile("/proc/" + e.Name() + "/stat")
		if err != nil {
			continue // process exited between ReadDir and read
		}
		if st, ok := parseProcStat(string(b), pageMB); ok {
			res[pid] = st
		}
	}
	return res
}

// parseProcStat parses one /proc/<pid>/stat line. comm may itself contain
// spaces and parens, so the line is split at the last ')'.
func parseProcStat(line string, pageMB float64) (procStat, bool) {
	open := strings.IndexByte(line, '(')
	close := strings.LastIndexByte(line, ')')
	if open < 0 || close < open {
		return procStat{}, false
	}
	comm := line[open+1 : close]
	fields := strings.Fields(line[close+1:])
	// fields[0] is man-page field 3 (state); utime/stime/rss are
	// fields 14, 15 and 24 → indices 11, 12 and 21 here.
	if len(fields) < 22 {
		return procStat{}, false
	}
	utime, _ := strconv.ParseUint(fields[11], 10, 64)
	stime, _ := strconv.ParseUint(fields[12], 10, 64)
	rssPages, _ := strconv.ParseFloat(fields[21], 64)
	return procStat{comm: comm, ticks: utime + stime, rssMB: rssPages * pageMB}, true
}
