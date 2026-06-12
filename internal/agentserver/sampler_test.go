package agentserver

import (
	"testing"
	"time"
)

func TestParseProcStat(t *testing.T) {
	// comm with spaces and parens must survive the split at the last ')'.
	line := "42 (tmux: server (1)) S 1 42 42 0 -1 4194304 500 0 0 0 " +
		"700 300 0 0 20 0 1 0 12345 104857600 2560 18446744073709551615 " +
		"0 0 0 0 0 0 0 0 0 0 0 0 17 3 0 0 0 0 0"
	st, ok := parseProcStat(line, 4096.0/(1024*1024))
	if !ok {
		t.Fatal("expected parse to succeed")
	}
	if st.comm != "tmux: server (1)" {
		t.Errorf("comm = %q", st.comm)
	}
	if st.ticks != 1000 { // utime 700 + stime 300
		t.Errorf("ticks = %d, want 1000", st.ticks)
	}
	if st.rssMB != 10 { // 2560 pages * 4KiB
		t.Errorf("rssMB = %v, want 10", st.rssMB)
	}
}

func TestParseProcStatRejectsGarbage(t *testing.T) {
	for _, line := range []string{"", "no parens here", "1 (short) S 2 3"} {
		if _, ok := parseProcStat(line, 1); ok {
			t.Errorf("expected parse failure for %q", line)
		}
	}
}

func TestBusyPct(t *testing.T) {
	a := cpuTimes{idle: 100, total: 200}
	b := cpuTimes{idle: 150, total: 300} // 50 of 100 new ticks idle
	if got := busyPct(a, b); got != 50 {
		t.Errorf("busyPct = %v, want 50", got)
	}
	if got := busyPct(b, b); got != 0 {
		t.Errorf("zero delta should yield 0, got %v", got)
	}
}

func TestSamplerPublishesImmediately(t *testing.T) {
	s := NewSampler()
	s.Start()
	defer s.Stop()

	if s.Latest().CollectedAt == "" {
		t.Error("Latest() right after Start() must have CollectedAt set")
	}
	if len(s.History()) == 0 {
		t.Error("history should contain the initial point")
	}

	// First real sample lands ~150ms after Start, adding a history point.
	time.Sleep(300 * time.Millisecond)
	if got := len(s.History()); got < 2 {
		t.Errorf("expected a fresh sample after the priming window, history has %d point(s)", got)
	}
}
