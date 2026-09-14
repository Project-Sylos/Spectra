package utils

import "testing"

func TestPathDepth(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"/", 0},
		{"", 0},
		{".", 0},
		{"/a", 1},
		{"/a/b", 2},
		{"/a/b/c", 3},
		{"a/b", 2},
	}
	for _, tc := range cases {
		if got := PathDepth(tc.in); got != tc.want {
			t.Errorf("PathDepth(%q) = %d; want %d", tc.in, got, tc.want)
		}
	}
}
