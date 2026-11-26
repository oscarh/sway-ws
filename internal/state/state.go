// Package state keeps per-project workspace history in a small JSON file.
package state

import (
	"slices"

	"github.com/oscarh/sway-ws/internal/wsname"
)

type Project struct {
	Last string `json:"last,omitempty"`
	Prev string `json:"prev,omitempty"`
}

type State struct {
	Projects    map[string]*Project `json:"projects"`
	Current     string              `json:"current,omitempty"`
	ProjectPrev string              `json:"project_prev,omitempty"`
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
	if s.Current != "" && s.Current != proj {
		s.ProjectPrev = s.Current
	}
	s.Current = proj
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

func (s *State) ProjectToggleTarget(names []string) (string, bool) {
	if s.ProjectPrev == "" || s.ProjectPrev == s.Current {
		return "", false
	}
	return s.ProjectTarget(s.ProjectPrev, names)
}

func (s *State) Rename(from, to string) {
	for _, p := range s.Projects {
		if p.Last == from {
			p.Last = to
		}
		if p.Prev == from {
			p.Prev = to
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
}
