// Package wsname implements the "<project>:<n>[:<label>]" workspace naming
// convention.
package wsname

import (
	"slices"
	"strconv"
	"strings"
	"unicode"
)

type Name struct {
	Raw     string
	Project string
	// Num is the slot within the project, 0 if the name has none.
	Num   int
	Label string
}

// Main is the project of workspaces without a project prefix.
const Main = "main"

// Parse never fails: a name without a colon is an unnumbered workspace in
// Main, which keeps workspaces created outside the convention usable.
func Parse(s string) Name {
	n := Name{Raw: s, Project: s}
	parts := strings.SplitN(s, ":", 3)
	if len(parts) == 1 {
		n.Project = Main
		n.Label = s
		return n
	}
	n.Project = parts[0]
	if num, err := strconv.Atoi(parts[1]); err == nil && num > 0 {
		n.Num = num
		if len(parts) == 3 {
			n.Label = parts[2]
		}
		return n
	}
	n.Label = strings.Join(parts[1:], ":")
	return n
}

func Format(project string, num int) string {
	return project + ":" + strconv.Itoa(num)
}

// Short is the name without the project, as shown in the bar.
func (n Name) Short() string {
	switch {
	case n.Num > 0 && n.Label != "":
		return strconv.Itoa(n.Num) + ":" + n.Label
	case n.Num > 0:
		return strconv.Itoa(n.Num)
	case n.Label != "":
		return n.Label
	default:
		return n.Raw
	}
}

// Compare orders by project, then numbered slots ascending, then unnumbered
// workspaces by name.
func Compare(a, b Name) int {
	if c := strings.Compare(a.Project, b.Project); c != 0 {
		return c
	}
	switch {
	case a.Num > 0 && b.Num == 0:
		return -1
	case a.Num == 0 && b.Num > 0:
		return 1
	case a.Num != b.Num:
		return a.Num - b.Num
	}
	return strings.Compare(a.Raw, b.Raw)
}

// InProject returns the workspaces of project, sorted.
func InProject(names []string, project string) []Name {
	var res []Name
	for _, s := range names {
		if n := Parse(s); n.Project == project {
			res = append(res, n)
		}
	}
	slices.SortFunc(res, Compare)
	return res
}

// Slot returns the workspace for slot num in project, creating the name if
// no such workspace exists.
func Slot(names []string, project string, num int) string {
	for _, n := range InProject(names, project) {
		if n.Num == num {
			return n.Raw
		}
	}
	return Format(project, num)
}

// FreeSlot is the lowest slot number not used in project.
func FreeSlot(names []string, project string) int {
	used := map[int]bool{}
	for _, n := range InProject(names, project) {
		used[n.Num] = true
	}
	n := 1
	for used[n] {
		n++
	}
	return n
}

func Projects(names []string) []string {
	var res []string
	for _, s := range names {
		p := Parse(s).Project
		if !slices.Contains(res, p) {
			res = append(res, p)
		}
	}
	slices.Sort(res)
	return res
}

// Others lists every project except current, alphabetically so that each
// keeps its place in the bar.
func Others(names []string, current string) []string {
	var res []string
	for _, p := range Projects(names) {
		if p != current {
			res = append(res, p)
		}
	}
	return res
}

// CleanLabel drops characters that sway's command parser or the bar treat
// specially, and surrounding space.
func CleanLabel(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || strings.ContainsRune(`"'\;,`, r) {
			return -1
		}
		return r
	}, s)
	return strings.TrimSpace(s)
}

// WithLabel is n renamed to carry label, keeping its project and number. An
// empty label clears it; an unnumbered workspace then gets the lowest free
// slot, since "project:" alone is not a name.
func WithLabel(n Name, label string, names []string) string {
	switch {
	case n.Num > 0 && label == "":
		return Format(n.Project, n.Num)
	case n.Num > 0:
		return Format(n.Project, n.Num) + ":" + label
	case !strings.Contains(n.Raw, ":"):
		// A workspace in Main by lack of a prefix: the whole name is its
		// label, and it must stay without a colon to stay in Main.
		if label == "" || strings.Contains(label, ":") {
			return n.Raw
		}
		return label
	case label == "":
		return Format(n.Project, FreeSlot(names, n.Project))
	default:
		return n.Project + ":" + label
	}
}
