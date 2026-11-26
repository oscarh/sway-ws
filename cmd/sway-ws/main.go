package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/oscarh/sway-ws/internal/ipc"
	"github.com/oscarh/sway-ws/internal/nav"
	"github.com/oscarh/sway-ws/internal/state"
	"github.com/oscarh/sway-ws/internal/wsname"
)

var version = "dev"

const usage = `usage: sway-ws <command>

  focus N         focus workspace N of the current project
  move N          move the focused window to workspace N of the current project
  toggle          focus the previously used workspace of the current project
  project-toggle  focus the last used workspace of the previous project
  pick            pick a project or workspace with wmenu and focus it
  pick-move       pick a project or workspace with wmenu and move the window there
  rename          rename the focused workspace
  nth N           focus the N-th workspace of the current project (bar clicks)
  other N         go to the N-th other project, alphabetically (bar clicks)

  waybar          waybar module: current project, streamed; also signals
                  waybar (SIGRTMIN+8) to refresh the modules below
  waybar-ws N     waybar module: N-th workspace of the current project
  waybar-more N   waybar module: count of workspaces beyond the first N
  waybar-other N  waybar module: N-th other project
  version         print the version
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if err := run(os.Args[1], os.Args[2:]); err != nil {
		fmt.Fprintln(os.Stderr, "sway-ws:", err)
		os.Exit(1)
	}
}

func run(cmd string, args []string) error {
	switch cmd {
	case "focus", "move":
		n, err := slotArg(args)
		if err != nil {
			return err
		}
		return withSession(func(s *session) error {
			ws := wsname.Slot(s.names, s.st.Current, n)
			if cmd == "move" {
				return s.move(ws)
			}
			return s.focus(ws)
		})
	case "toggle":
		return withSession(func(s *session) error {
			if ws, ok := s.st.ToggleTarget(s.names); ok {
				return s.focus(ws)
			}
			return nil
		})
	case "project-toggle":
		return withSession(func(s *session) error {
			if ws, ok := s.st.ProjectToggleTarget(s.names); ok {
				return s.focus(ws)
			}
			return nil
		})
	case "pick", "pick-move":
		return pick(cmd == "pick-move")
	case "rename":
		return rename()
	case "nth":
		n, err := slotArg(args)
		if err != nil {
			return err
		}
		return withSession(func(s *session) error {
			ws := wsname.InProject(s.names, s.st.Current)
			if n > len(ws) {
				return nil
			}
			return s.focus(ws[n-1].Raw)
		})
	case "other":
		n, err := slotArg(args)
		if err != nil {
			return err
		}
		return withSession(func(s *session) error {
			others := wsname.Others(s.names, s.st.Current)
			if n > len(others) {
				return nil
			}
			if ws, ok := s.st.ProjectTarget(others[n-1], s.names); ok {
				return s.focus(ws)
			}
			return nil
		})
	case "waybar":
		return waybarProject()
	case "waybar-ws", "waybar-more", "waybar-other":
		n, err := slotArg(args)
		if err != nil {
			return err
		}
		return waybarOnce(cmd, n)
	case "version":
		fmt.Println(version)
		return nil
	case "help", "-h", "--help":
		fmt.Print(usage)
		return nil
	}
	return fmt.Errorf("unknown command %q\n%s", cmd, usage)
}

func slotArg(args []string) (int, error) {
	if len(args) != 1 {
		return 0, errors.New("expected exactly one workspace number")
	}
	n, err := strconv.Atoi(args[0])
	if err != nil || n < 1 {
		return 0, fmt.Errorf("invalid workspace number %q", args[0])
	}
	return n, nil
}

// session holds the state lock for the duration of one command.
type session struct {
	conn    *ipc.Conn
	file    *state.File
	st      *state.State
	names   []string
	visible []string
	// start is the project that had the focus when the command started.
	start string
}

// withSession first records the focused workspace, so switches made outside
// sway-ws (mouse, focus stealing) still end up in the history.
func withSession(fn func(*session) error) error {
	sock, err := ipc.SocketPath()
	if err != nil {
		return err
	}
	path, err := state.Path(sock)
	if err != nil {
		return err
	}
	conn, err := ipc.Dial()
	if err != nil {
		return err
	}
	defer conn.Close()
	file, err := state.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	wss, err := conn.Workspaces()
	if err != nil {
		return err
	}
	s := &session{conn: conn, file: file, st: file.Load(), names: ipc.Names(wss)}
	for _, w := range wss {
		if w.Visible {
			s.visible = append(s.visible, w.Name)
		}
	}
	if f, ok := ipc.Focused(wss); ok {
		s.st.Observe(f.Name)
	}
	s.start = s.st.Current
	var mine []string
	for _, ws := range s.visible {
		if wsname.Parse(ws).Project == s.start {
			mine = append(mine, ws)
		}
	}
	s.st.Snapshot(mine)

	fnErr := fn(s)

	if wss, err := conn.Workspaces(); err == nil {
		s.st.Prune(ipc.Names(wss))
	}
	return errors.Join(fnErr, file.Save(s.st))
}

// focus goes to ws. Entering another project first brings back the
// workspaces it had on the other outputs, all in one sway command so the
// outputs switch together.
func (s *session) focus(ws string) error {
	var cmds []string
	if p := wsname.Parse(ws).Project; p != s.start {
		for _, r := range s.st.Restore(p, ws, s.names, s.visible) {
			cmds = append(cmds, "workspace "+ipc.Quote(r))
		}
	}
	cmds = append(cmds, "workspace "+ipc.Quote(ws))
	if err := s.conn.Command(strings.Join(cmds, "; ")); err != nil {
		return err
	}
	s.st.Observe(ws)
	return nil
}

func (s *session) move(ws string) error {
	return s.conn.Command("move container to workspace " + ipc.Quote(ws))
}

// menu returns an empty string if the user aborted.
func menu(prompt string, items []string) (string, error) {
	cmd := exec.Command("wmenu", "-b", "-i", "-p", prompt)
	cmd.Stdin = strings.NewReader(strings.Join(items, "\n") + "\n")
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func currentNames() ([]ipc.Workspace, error) {
	conn, err := ipc.Dial()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	return conn.Workspaces()
}

// The menu runs before taking the state lock so that an open picker does not
// block other bindings.
func pick(moveWindow bool) error {
	wss, err := currentNames()
	if err != nil {
		return err
	}
	prompt := "workspace"
	if moveWindow {
		prompt = "move to"
	}
	choice, err := menu(prompt, nav.PickItems(ipc.Names(wss)))
	if err != nil || choice == "" {
		return err
	}
	return withSession(func(s *session) error {
		ws := nav.PickTarget(s.st, s.names, choice)
		if ws == "" {
			return nil
		}
		if moveWindow {
			return s.move(ws)
		}
		return s.focus(ws)
	})
}

func rename() error {
	wss, err := currentNames()
	if err != nil {
		return err
	}
	focused, ok := ipc.Focused(wss)
	if !ok {
		return nil
	}
	// Tab in wmenu copies the item into the input, which gives an editable
	// prefill of the current name.
	to, err := menu("rename "+focused.Name+" to", []string{focused.Name})
	if err != nil || to == "" || to == focused.Name {
		return err
	}
	return withSession(func(s *session) error {
		cmd := "rename workspace " + ipc.Quote(focused.Name) + " to " + ipc.Quote(to)
		if err := s.conn.Command(cmd); err != nil {
			return err
		}
		s.st.Rename(focused.Name, to)
		if wss, err := s.conn.Workspaces(); err == nil {
			if f, ok := ipc.Focused(wss); ok {
				s.st.Observe(f.Name)
			}
		}
		return nil
	})
}
