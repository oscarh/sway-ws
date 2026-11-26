// Package render produces the output of the waybar custom modules: one
// module for the current project and one per workspace position in it.
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
	project string
	byName  map[string]ipc.Workspace
	names   []string
	current []wsname.Name
}

func newView(wss []ipc.Workspace) (view, bool) {
	focused, ok := ipc.Focused(wss)
	if !ok {
		return view{}, false
	}
	v := view{
		project: wsname.Parse(focused.Name).Project,
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
func Project(wss []ipc.Workspace) Output {
	v, ok := newView(wss)
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
func Other(wss []ipc.Workspace, pos int) Output {
	v, ok := newView(wss)
	if !ok {
		return Output{}
	}
	others := wsname.Others(v.names, v.project)
	if pos < 1 || pos > len(others) {
		return Output{}
	}
	p := others[pos-1]
	class := []string{"other"}
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
func Position(wss []ipc.Workspace, pos int) Output {
	v, ok := newView(wss)
	if !ok || pos < 1 || pos > len(v.current) {
		return Output{}
	}
	n := v.current[pos-1]
	w := v.byName[n.Raw]
	class := []string{"ws"}
	switch {
	case w.Urgent:
		class = append(class, "urgent")
	case w.Focused:
		class = append(class, "focused")
	case w.Visible:
		class = append(class, "visible")
	}
	return Output{Text: html.EscapeString(n.Short()), Tooltip: html.EscapeString(n.Raw), Class: class}
}

// More counts the workspaces of the current project beyond the first shown
// positions.
func More(wss []ipc.Workspace, shown int) Output {
	v, ok := newView(wss)
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
