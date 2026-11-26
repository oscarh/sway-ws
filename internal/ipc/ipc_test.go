package ipc

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

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

func TestRequestTimesOutWhenSwayDoesNotAnswer(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "sway.sock")
	l, err := net.Listen("unix", sock)
	assert.NilError(t, err)
	defer l.Close()
	go func() {
		c, err := l.Accept()
		if err == nil {
			defer c.Close()
			time.Sleep(2 * Timeout)
		}
	}()
	t.Setenv("SWAYSOCK", sock)

	c, err := Dial()
	assert.NilError(t, err)
	defer c.Close()
	start := time.Now()
	_, err = c.Workspaces()
	assert.Assert(t, errors.Is(err, os.ErrDeadlineExceeded), "got %v", err)
	assert.Assert(t, time.Since(start) < Timeout+time.Second)
}
