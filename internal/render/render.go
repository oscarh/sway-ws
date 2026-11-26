// Package render produces the output of the waybar custom modules: one
// module for the project, one per workspace position in it and one per
// other project. Each bar renders from the point of view of its output:
// the project is the one of the workspace shown there.
package render

import (
	"html"
	"strconv"
	"strings"

	"github.com/oscarh/sway-ws/internal/ipc"
	"github.com/oscarh/sway-ws/internal/wsname"
)

type Output struct {
	Text    string   `json:"text"`
	Tooltip string   `json:"tooltip,omitempty"`
	Class   []string `json:"class,omitempty"`
}

type view struct {
	output  string
	project string
	byName  map[string]ipc.Workspace
	names   []string
	current []wsname.Name
}

// newView is the bar on output; an empty output means the focused one.
func newView(wss []ipc.Workspace, output string) (view, bool) {
	here, ok := ipc.OnOutput(wss, output)
	if !ok {
		return view{}, false
	}
	v := view{
		output:  here.Output,
		project: wsname.Parse(here.Name).Project,
		byName:  make(map[string]ipc.Workspace, len(wss)),
		names:   ipc.Names(wss),
	}
	for _, w := range wss {
		v.byName[w.Name] = w
	}
	v.current = wsname.InProject(v.names, v.project)
	return v, true
}

// Project is the box in front of the workspaces.
func Project(wss []ipc.Workspace, output string) Output {
	v, ok := newView(wss, output)
	if !ok {
		return Output{}
	}
	var tooltip []string
	for _, p := range wsname.Projects(v.names) {
		tooltip = append(tooltip, v.summary(p))
	}
	return Output{
		Text:    html.EscapeString(v.project),
		Tooltip: html.EscapeString(strings.Join(tooltip, "\n")),
		Class:   []string{"project"},
	}
}

func (v view) summary(project string) string {
	var short []string
	for _, n := range wsname.InProject(v.names, project) {
		short = append(short, n.Short())
	}
	return project + ": " + strings.Join(short, " ")
}

// Other is the pos-th (1-based) project other than the current one. It is
// marked urgent if any of its workspaces is, which is the only way to see
// urgency in a project that is not shown.
func Other(wss []ipc.Workspace, output string, pos int) Output {
	v, ok := newView(wss, output)
	if !ok {
		return Output{}
	}
	others := wsname.Others(v.names, v.project)
	if pos < 1 || pos > len(others) {
		return Output{}
	}
	p := others[pos-1]
	class := []string{"other"}
	if pos == 1 {
		class = append(class, "first")
	}
	urgent, visible := false, false
	for _, n := range wsname.InProject(v.names, p) {
		urgent = urgent || v.byName[n.Raw].Urgent
		visible = visible || v.byName[n.Raw].Visible
	}
	switch {
	case urgent:
		class = append(class, "urgent")
	case visible:
		class = append(class, "visible")
	}
	return Output{
		Text:    html.EscapeString(p),
		Tooltip: html.EscapeString(v.summary(p)),
		Class:   class,
	}
}

// Position is the pos-th (1-based) workspace of the current project, or an
// empty output, which makes waybar hide the module.
func Position(wss []ipc.Workspace, output string, pos int) Output {
	v, ok := newView(wss, output)
	if !ok || pos < 1 || pos > len(v.current) {
		return Output{}
	}
	n := v.current[pos-1]
	w := v.byName[n.Raw]
	class := []string{"ws"}
	switch {
	case w.Urgent:
		class = append(class, "urgent")
	// Focused and active are about this bar's output only: the focused
	// workspace on another output is just visible from here.
	case w.Visible && w.Output == v.output && w.Focused:
		class = append(class, "focused")
	case w.Visible && w.Output == v.output:
		// Shown on this bar's output, while another output has the focus.
		class = append(class, "active")
	case w.Visible:
		class = append(class, "visible")
	}
	return Output{Text: html.EscapeString(n.Short()), Tooltip: html.EscapeString(n.Raw), Class: class}
}

// More counts the workspaces of the current project beyond the first shown
// positions.
func More(wss []ipc.Workspace, output string, shown int) Output {
	v, ok := newView(wss, output)
	if !ok || len(v.current) <= shown {
		return Output{}
	}
	hidden := v.current[shown:]
	var short []string
	class := []string{"ws", "more"}
	urgent := false
	for _, n := range hidden {
		short = append(short, n.Short())
		urgent = urgent || v.byName[n.Raw].Urgent
	}
	if urgent {
		class = append(class, "urgent")
	}
	return Output{
		Text:    "+" + strconv.Itoa(len(hidden)),
		Tooltip: html.EscapeString(strings.Join(short, "\n")),
		Class:   class,
	}
}
