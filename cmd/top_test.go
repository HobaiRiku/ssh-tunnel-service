package cmd

import "testing"

func TestHumanBytes(t *testing.T) {
	cases := map[uint64]string{
		0:       "0 B",
		1023:    "1023 B",
		1024:    "1.0 KiB",
		1536:    "1.5 KiB",
		5 << 20: "5.0 MiB",
		3 << 30: "3.0 GiB",
	}
	for in, want := range cases {
		if got := humanBytes(in); got != want {
			t.Errorf("humanBytes(%d) = %q, want %q", in, got, want)
		}
	}
	if got := humanRate(2048.4); got != "2.0 KiB/s" {
		t.Errorf("humanRate = %q", got)
	}
}
