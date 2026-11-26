// Package state keeps per-project workspace history in a small JSON file.
package state

import (
	"slices"

	"github.com/oscarh/sway-ws/internal/wsname"
)

type Project struct {
	Last string `json:"last,omitempty"`
	Prev string `json:"prev,omitempty"`
	// Visible are the project's workspaces that were on screen, on any
	// output, the last time it had the focus.
	Visible []string `json:"visible,omitempty"`
}

type State struct {
	Projects map[string]*Project `json:"projects"`
	Current  string              `json:"current,omitempty"`
	// Recent lists projects, most recently focused first.
	Recent []string `json:"recent,omitempty"`
}

func New() *State {
	return &State{Projects: map[string]*Project{}}
}

func (s *State) project(name string) *Project {
	if s.Projects == nil {
		s.Projects = map[string]*Project{}
	}
	p, ok := s.Projects[name]
	if !ok {
		p = &Project{}
		s.Projects[name] = p
	}
	return p
}

// Observe records that ws is now focused.
func (s *State) Observe(ws string) {
	proj := wsname.Parse(ws).Project
	s.Current = proj
	if len(s.Recent) == 0 || s.Recent[0] != proj {
		s.Recent = slices.DeleteFunc(s.Recent, func(r string) bool { return r == proj })
		s.Recent = slices.Insert(s.Recent, 0, proj)
	}
	p := s.project(proj)
	if p.Last != ws {
		p.Prev = p.Last
		p.Last = ws
	}
}

// valid reports whether ws still exists and still belongs to project; a
// rename can move a remembered workspace to another project.
func valid(ws, project string, names []string) bool {
	return ws != "" && slices.Contains(names, ws) && wsname.Parse(ws).Project == project
}

// ProjectTarget is where entering project should land: the workspace last
// used there, or its lowest-numbered workspace.
func (s *State) ProjectTarget(project string, names []string) (string, bool) {
	if p, ok := s.Projects[project]; ok && valid(p.Last, project, names) {
		return p.Last, true
	}
	if ws := wsname.InProject(names, project); len(ws) > 0 {
		return ws[0].Raw, true
	}
	return "", false
}

// ToggleTarget is the previously used workspace in the current project.
func (s *State) ToggleTarget(names []string) (string, bool) {
	p, ok := s.Projects[s.Current]
	if !ok || !valid(p.Prev, s.Current, names) {
		return "", false
	}
	return p.Prev, true
}

// ProjectToggleTarget is the workspace to land on in the most recent
// project other than the current one. The home project is passed over, so
// a look at it does not come between the projects being toggled; it is
// used only when there is no other project.
func (s *State) ProjectToggleTarget(names []string, home string) (string, bool) {
	for _, p := range s.Recent {
		if p == s.Current || p == home {
			continue
		}
		if ws, ok := s.ProjectTarget(p, names); ok {
			return ws, true
		}
	}
	if s.Current != home {
		return s.ProjectTarget(home, names)
	}
	return "", false
}

// HomeTarget goes to the home project, or back from it to the project
// Alt+Tab would go to. A home project without workspaces gets its first.
func (s *State) HomeTarget(names []string, home string) (string, bool) {
	if s.Current == home {
		return s.ProjectToggleTarget(names, home)
	}
	if ws, ok := s.ProjectTarget(home, names); ok {
		return ws, true
	}
	return wsname.Format(home, 1), true
}

// Snapshot records which workspaces of the current project are on screen.
// An empty set is ignored: then the project is not on screen at all, and
// the previous record is the better one to restore.
func (s *State) Snapshot(visible []string) {
	if len(visible) == 0 || s.Current == "" {
		return
	}
	s.project(s.Current).Visible = slices.Clone(visible)
}

// Restore lists the workspaces to show before focusing target when entering
// project, so that the project comes back on every output it was on.
// Workspaces that are gone, have moved to another project, or are already
// on screen are skipped.
func (s *State) Restore(project, target string, names, visibleNow []string) []string {
	p, ok := s.Projects[project]
	if !ok {
		return nil
	}
	var res []string
	for _, ws := range p.Visible {
		if ws != target && valid(ws, project, names) && !slices.Contains(visibleNow, ws) {
			res = append(res, ws)
		}
	}
	return res
}

func (s *State) Rename(from, to string) {
	for _, p := range s.Projects {
		if p.Last == from {
			p.Last = to
		}
		if p.Prev == from {
			p.Prev = to
		}
		for i, ws := range p.Visible {
			if ws == from {
				p.Visible[i] = to
			}
		}
	}
}

// Prune drops projects that no longer have any workspaces so the file does
// not grow forever.
func (s *State) Prune(names []string) {
	live := wsname.Projects(names)
	for name := range s.Projects {
		if !slices.Contains(live, name) {
			delete(s.Projects, name)
		}
	}
	s.Recent = slices.DeleteFunc(s.Recent, func(p string) bool { return !slices.Contains(live, p) })
}
