package main

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestRTMinFromMask(t *testing.T) {
	cases := []struct {
		name string
		mask uint64
		want int
		ok   bool
	}{
		// From a glibc-built waybar: 35..64 handled, plus glibc's own 32, 33.
		{"glibc", 0xfffffffd80010a02, 34, true},
		{"musl", 0xfffffff800010a02, 35, true},
		{"no real-time handlers", 0x0000000000010a02, 0, false},
		{"SIGRTMAX not handled", 0x7ffffffd80010a02, 0, false},
		{"too few for signal 8", 0xf800000000010a02, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := rtminFromMask(c.mask)
			assert.Equal(t, ok, c.ok)
			assert.Equal(t, got, c.want)
		})
	}
}

func TestWaybarRTMinMissingProcess(t *testing.T) {
	_, err := waybarRTMin(1 << 30)
	assert.ErrorContains(t, err, "no such file")
}
