package worker

import "testing"

func TestNumWorkers(t *testing.T) {
	tests := []struct {
		value string
		want  int
	}{
		{value: "", want: defaultNumWorkers},
		{value: "invalid", want: defaultNumWorkers},
		{value: "0", want: defaultNumWorkers},
		{value: "-1", want: defaultNumWorkers},
		{value: "1", want: 1},
		{value: "8", want: 8},
	}

	for _, tt := range tests {
		t.Setenv("NUM_WORKERS", tt.value)
		if have := numWorkers(); have != tt.want {
			t.Fatalf("NUM_WORKERS=%q: want %d, have %d", tt.value, tt.want, have)
		}
	}
}
