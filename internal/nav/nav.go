// Package nav decides which workspace a picker selection refers to.
package nav

import (
	"slices"
	"strings"

	"github.com/oscarh/sway-ws/internal/state"
	"github.com/oscarh/sway-ws/internal/wsname"
)

// PickItems lists project names first, then every workspace. A project whose
// name is also a workspace name (old-style workspaces) is listed only once.
func PickItems(names []string) []string {
	var items []string
	for _, p := range wsname.Projects(names) {
		if !slices.Contains(names, p) {
			items = append(items, p)
		}
	}
	parsed := make([]wsname.Name, len(names))
	for i, n := range names {
		parsed[i] = wsname.Parse(n)
	}
	slices.SortFunc(parsed, wsname.Compare)
	for _, n := range parsed {
		items = append(items, n.Raw)
	}
	return items
}

// PickTarget resolves what the user selected or typed in the picker:
//   - an existing workspace is used as is
//   - an existing project lands on the workspace last used there
//   - "proj:" (trailing colon) does the same, or creates proj:1
//   - any other bare name becomes a new workspace in the current project,
//     in its lowest free slot
//   - anything else with a colon is created as typed
func PickTarget(st *state.State, names []string, input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}
	if slices.Contains(names, input) {
		return input
	}
	if p, ok := strings.CutSuffix(input, ":"); ok && p != "" && !strings.Contains(p, ":") {
		if ws, ok := st.ProjectTarget(p, names); ok {
			return ws
		}
		return wsname.Format(p, 1)
	}
	if strings.Contains(input, ":") {
		return input
	}
	if ws, ok := st.ProjectTarget(input, names); ok {
		return ws
	}
	cur := st.Current
	if cur == "" {
		cur = wsname.Main
	}
	return wsname.Format(cur, wsname.FreeSlot(names, cur)) + ":" + input
}
