package httpx

import "testing"

func TestSafeFilename(t *testing.T) {
	tests := []struct {
		name     string
		fallback string
		want     string
	}{
		{name: "a.csv", fallback: "res_1", want: "a.csv"},
		{name: "a/b.csv", fallback: "res_1", want: "a_b.csv"},
		{name: `a\b.csv`, fallback: "res_1", want: "a_b.csv"},
		{name: "a\r\nb.csv", fallback: "res_1", want: "ab.csv"},
		{name: "", fallback: "res_1", want: "res_1"},
		{name: ".", fallback: "res_1", want: "res_1"},
		{name: "..", fallback: "res_1", want: "res_1"},
		{name: "", fallback: "", want: "download"},
		{name: "", fallback: ".", want: "download"},
	}
	for _, tt := range tests {
		if got := SafeFilename(tt.name, tt.fallback); got != tt.want {
			t.Fatalf("SafeFilename(%q, %q) = %q, want %q", tt.name, tt.fallback, got, tt.want)
		}
	}
}
