package cli

import "testing"

func TestParseVRAM(t *testing.T) {
	tests := []struct {
		input string
		want  uint64
	}{
		{"0", 0},
		{"", 0},
		{"40g", 40000},
		{"40gb", 40000},
		{"40G", 40000},
		{"512m", 512},
		{"512mb", 512},
		{"1000", 1000},
		{"  40g  ", 40000},
	}

	for _, tt := range tests {
		got := parseVRAM(tt.input)
		if got != tt.want {
			t.Errorf("parseVRAM(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
