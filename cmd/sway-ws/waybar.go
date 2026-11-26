package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/oscarh/sway-ws/internal/ipc"
	"github.com/oscarh/sway-ws/internal/render"
)

// The workspace modules use "signal": 8 in the waybar config; waybar maps it
// to SIGRTMIN+8, and glibc's SIGRTMIN is 34.
const refreshSignal = syscall.Signal(34 + 8)

// A focus change produces a burst of workspace events (focus, empty, init);
// coalescing them keeps the bar from re-running every module several times.
const debounce = 15 * time.Millisecond

func writeJSON(w *bufio.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return err
	}
	return w.Flush()
}

func waybarOnce(cmd string, n int) error {
	conn, err := ipc.Dial()
	if err != nil {
		return err
	}
	defer conn.Close()
	wss, err := conn.Workspaces()
	if err != nil {
		return err
	}
	output := os.Getenv("WAYBAR_OUTPUT_NAME")
	var out render.Output
	switch cmd {
	case "waybar-more":
		out = render.More(wss, output, n)
	case "waybar-other":
		out = render.Other(wss, output, n)
	default:
		out = render.Position(wss, output, n)
	}
	return writeJSON(bufio.NewWriter(os.Stdout), out)
}

// signaller decides which of the project modules signals waybar. All bars
// live in one waybar process, and each has its own project module; without
// this every change would refresh every module once per bar.
type signaller struct {
	bar  int
	path string
	lock *os.File
}

func (s *signaller) signal() {
	if s.bar <= 0 {
		return
	}
	// Retried on every change so another module takes over if the holder
	// is restarted.
	if s.lock == nil {
		f, err := os.OpenFile(s.path, os.O_CREATE|os.O_RDWR, 0o600)
		if err != nil {
			return
		}
		if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			f.Close()
			return
		}
		s.lock = f
	}
	_ = syscall.Kill(s.bar, refreshSignal)
}

func newSignaller() *signaller {
	s := &signaller{bar: waybarAncestor()}
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if s.bar <= 0 || dir == "" {
		s.bar = 0
		return s
	}
	dir = filepath.Join(dir, "sway-ws")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		s.bar = 0
		return s
	}
	s.path = filepath.Join(dir, "waybar-"+strconv.Itoa(s.bar)+".lock")
	removeStaleLocks(dir)
	return s
}

// removeStaleLocks deletes the lock files of waybar processes that are gone,
// one is left behind by every waybar restart.
func removeStaleLocks(dir string) {
	paths, _ := filepath.Glob(filepath.Join(dir, "waybar-*.lock"))
	for _, p := range paths {
		pid := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(p), "waybar-"), ".lock")
		if _, err := os.Stat("/proc/" + pid); os.IsNotExist(err) {
			os.Remove(p)
		}
	}
}

// waybarProject streams the project module and tells its own waybar to
// re-run the per-workspace modules.
func waybarProject() error {
	// Exit together with waybar instead of at the next workspace change.
	// This needs waybar to be the parent, i.e. "exec" in the module's
	// command, and costs nothing while running.
	parent := os.Getppid()
	if _, _, errno := syscall.RawSyscall(syscall.SYS_PRCTL, syscall.PR_SET_PDEATHSIG, uintptr(syscall.SIGTERM), 0); errno != 0 {
		return errno
	}
	if os.Getppid() != parent {
		return nil
	}

	events, err := ipc.Dial()
	if err != nil {
		return err
	}
	defer events.Close()
	if err := events.Subscribe("workspace"); err != nil {
		return err
	}
	query, err := ipc.Dial()
	if err != nil {
		return err
	}
	defer query.Close()

	sig := newSignaller()
	output := os.Getenv("WAYBAR_OUTPUT_NAME")
	out := bufio.NewWriter(os.Stdout)
	emit := func() error {
		wss, err := query.Workspaces()
		if err != nil {
			return err
		}
		if err := writeJSON(out, render.Project(wss, output)); err != nil {
			return err
		}
		sig.signal()
		return nil
	}

	changed := make(chan struct{}, 1)
	readErr := make(chan error, 1)
	go func() {
		for {
			typ, _, err := events.Read()
			if err != nil {
				readErr <- err
				return
			}
			if typ == ipc.EventWorkspace {
				select {
				case changed <- struct{}{}:
				default:
				}
			}
		}
	}()

	if err := emit(); err != nil {
		return err
	}
	for {
		select {
		case err := <-readErr:
			return err
		case <-changed:
		}
		time.Sleep(debounce)
		select {
		case <-changed:
		default:
		}
		if err := emit(); err != nil {
			return err
		}
	}
}

// waybarAncestor walks up the process tree, since waybar starts modules
// through "sh -c".
func waybarAncestor() int {
	pid := os.Getppid()
	for pid > 1 {
		comm, ppid, ok := procStat(pid)
		if !ok {
			return 0
		}
		if comm == "waybar" {
			return pid
		}
		pid = ppid
	}
	return 0
}

func procStat(pid int) (string, int, bool) {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return "", 0, false
	}
	s := string(data)
	// comm is in parentheses and may itself contain spaces or parentheses.
	open, end := strings.IndexByte(s, '('), strings.LastIndexByte(s, ')')
	if open < 0 || end < open {
		return "", 0, false
	}
	fields := strings.Fields(s[end+1:])
	if len(fields) < 2 {
		return "", 0, false
	}
	ppid, err := strconv.Atoi(fields[1])
	if err != nil {
		return "", 0, false
	}
	return s[open+1 : end], ppid, true
}
