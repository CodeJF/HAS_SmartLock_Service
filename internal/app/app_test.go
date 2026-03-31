package app

import "testing"

func TestGinMode(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{name: "production", input: "production", expect: "release"},
		{name: "test", input: "test", expect: "test"},
		{name: "default", input: "development", expect: "debug"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ginMode(tt.input); got != tt.expect {
				t.Fatalf("ginMode(%q) = %q, want %q", tt.input, got, tt.expect)
			}
		})
	}
}
