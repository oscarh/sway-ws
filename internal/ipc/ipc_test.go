package ipc

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestOnOutput(t *testing.T) {
	wss := []Workspace{
		{Name: "a:1", Output: "eDP-1", Visible: true},
		{Name: "a:2", Output: "eDP-1"},
		{Name: "b:1", Output: "DP-2", Visible: true},
		{Name: "c:1", Output: "DP-1", Visible: true, Focused: true},
	}
	cases := map[string]string{
		"":       "c:1",
		"eDP-1":  "a:1",
		"DP-2":   "b:1",
		"!eDP-1": "c:1",
		"!DP-1":  "b:1",
		"HDMI-1": "c:1",
	}
	for sel, want := range cases {
		w, ok := OnOutput(wss, sel)
		assert.Assert(t, ok)
		assert.Equal(t, w.Name, want, "selector %q", sel)
	}
	_, ok := OnOutput(nil, "eDP-1")
	assert.Assert(t, !ok)
}
