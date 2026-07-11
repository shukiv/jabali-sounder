package api

import "testing"

func TestCSVSafe(t *testing.T) {
	cases := map[string]string{
		"=cmd|' /C calc'!A0": "'=cmd|' /C calc'!A0",
		"+1":                 "'+1",
		"-2":                 "'-2",
		"@SUM(A1)":           "'@SUM(A1)",
		"normal.example.com": "normal.example.com",
		"":                   "",
	}
	for in, want := range cases {
		if got := csvSafe(in); got != want {
			t.Errorf("csvSafe(%q) = %q, want %q", in, got, want)
		}
	}
}
