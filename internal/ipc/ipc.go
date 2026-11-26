// Package ipc is a minimal client for the i3/sway IPC protocol.
package ipc

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"slices"
	"strings"
	"syscall"
	"time"
)

const (
	msgRunCommand    uint32 = 0
	msgGetWorkspaces uint32 = 1
	msgSubscribe     uint32 = 2

	EventWorkspace uint32 = 0x80000000
)

var magic = []byte("i3-ipc")

type Workspace struct {
	Num     int    `json:"num"`
	Name    string `json:"name"`
	Visible bool   `json:"visible"`
	Focused bool   `json:"focused"`
	Urgent  bool   `json:"urgent"`
	Output  string `json:"output"`
}

// Timeout bounds every request to sway, so that a hung sway cannot keep a
// command, and the state lock it holds, waiting forever.
const Timeout = 2 * time.Second

type Conn struct {
	c net.Conn
}

// SocketPath returns the socket sway exports to the processes it starts.
func SocketPath() (string, error) {
	for _, env := range []string{"SWAYSOCK", "I3SOCK"} {
		if p := os.Getenv(env); p != "" {
			return p, nil
		}
	}
	return "", errors.New("neither SWAYSOCK nor I3SOCK is set")
}

// Dial connects to sway. Sway keeps only a short queue of pending
// connections, and waybar starts all workspace modules at once, so a full
// queue (EAGAIN) is retried for up to Timeout rather than failing.
func Dial() (*Conn, error) {
	path, err := SocketPath()
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(Timeout)
	wait := 2 * time.Millisecond
	for {
		c, err := net.Dial("unix", path)
		if err == nil {
			return &Conn{c: c}, nil
		}
		if !errors.Is(err, syscall.EAGAIN) || time.Now().After(deadline) {
			return nil, err
		}
		time.Sleep(wait)
		wait = min(2*wait, 50*time.Millisecond)
	}
}

func (c *Conn) Close() error {
	return c.c.Close()
}

func (c *Conn) send(typ uint32, payload []byte) error {
	buf := make([]byte, 0, len(magic)+8+len(payload))
	buf = append(buf, magic...)
	buf = binary.NativeEndian.AppendUint32(buf, uint32(len(payload)))
	buf = binary.NativeEndian.AppendUint32(buf, typ)
	buf = append(buf, payload...)
	_, err := c.c.Write(buf)
	return err
}

// Read returns the next message, which is either a reply or an event.
func (c *Conn) Read() (uint32, []byte, error) {
	hdr := make([]byte, len(magic)+8)
	if _, err := io.ReadFull(c.c, hdr); err != nil {
		return 0, nil, err
	}
	if string(hdr[:len(magic)]) != string(magic) {
		return 0, nil, errors.New("bad ipc magic")
	}
	n := binary.NativeEndian.Uint32(hdr[len(magic):])
	typ := binary.NativeEndian.Uint32(hdr[len(magic)+4:])
	payload := make([]byte, n)
	if _, err := io.ReadFull(c.c, payload); err != nil {
		return 0, nil, err
	}
	return typ, payload, nil
}

func (c *Conn) roundTrip(typ uint32, payload []byte) ([]byte, error) {
	if err := c.c.SetDeadline(time.Now().Add(Timeout)); err != nil {
		return nil, err
	}
	// Cleared again so that a subscribed connection can wait for events.
	defer c.c.SetDeadline(time.Time{})
	if err := c.send(typ, payload); err != nil {
		return nil, err
	}
	// Events may be interleaved with replies on a subscribed connection.
	for {
		rtyp, reply, err := c.Read()
		if err != nil {
			return nil, err
		}
		if rtyp == typ {
			return reply, nil
		}
	}
}

func (c *Conn) Workspaces() ([]Workspace, error) {
	reply, err := c.roundTrip(msgGetWorkspaces, nil)
	if err != nil {
		return nil, err
	}
	var ws []Workspace
	if err := json.Unmarshal(reply, &ws); err != nil {
		return nil, err
	}
	return ws, nil
}

func (c *Conn) Command(cmd string) error {
	reply, err := c.roundTrip(msgRunCommand, []byte(cmd))
	if err != nil {
		return err
	}
	var results []struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(reply, &results); err != nil {
		return err
	}
	var errs []string
	for _, r := range results {
		if !r.Success {
			errs = append(errs, r.Error)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("sway command %q failed: %s", cmd, strings.Join(errs, "; "))
	}
	return nil
}

func (c *Conn) Subscribe(events ...string) error {
	payload, err := json.Marshal(events)
	if err != nil {
		return err
	}
	reply, err := c.roundTrip(msgSubscribe, payload)
	if err != nil {
		return err
	}
	var r struct {
		Success bool `json:"success"`
	}
	if err := json.Unmarshal(reply, &r); err != nil {
		return err
	}
	if !r.Success {
		return fmt.Errorf("subscribe to %v failed", events)
	}
	return nil
}

// Quote makes s safe to use as a single argument in a sway command.
func Quote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

func Focused(wss []Workspace) (Workspace, bool) {
	for _, w := range wss {
		if w.Focused {
			return w, true
		}
	}
	return Workspace{}, false
}

func Names(wss []Workspace) []string {
	names := make([]string, len(wss))
	for i, w := range wss {
		names[i] = w.Name
	}
	return names
}

// OnOutput is the workspace shown on the output that sel names, or the
// focused one if sel is empty or matches nothing. "!NAME" means any output
// other than NAME; the focused one wins, then the first by output name.
func OnOutput(wss []Workspace, sel string) (Workspace, bool) {
	if sel == "" {
		return Focused(wss)
	}
	not, negated := strings.CutPrefix(sel, "!")
	var match []Workspace
	for _, w := range wss {
		if !w.Visible {
			continue
		}
		if (negated && w.Output != not) || (!negated && w.Output == sel) {
			match = append(match, w)
		}
	}
	if len(match) == 0 {
		return Focused(wss)
	}
	slices.SortFunc(match, func(a, b Workspace) int {
		if a.Focused != b.Focused {
			if a.Focused {
				return -1
			}
			return 1
		}
		return strings.Compare(a.Output, b.Output)
	})
	return match[0], true
}
