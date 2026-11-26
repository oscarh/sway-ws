package wsname

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestParse(t *testing.T) {
	cases := []struct {
		in   string
		want Name
	}{
		{"home:1", Name{Raw: "home:1", Project: "home", Num: 1}},
		{"home:2:fw", Name{Raw: "home:2:fw", Project: "home", Num: 2, Label: "fw"}},
		{"home:2:a:b", Name{Raw: "home:2:a:b", Project: "home", Num: 2, Label: "a:b"}},
		{"slack", Name{Raw: "slack", Project: "main", Label: "slack"}},
		{"home:fw", Name{Raw: "home:fw", Project: "home", Label: "fw"}},
		{"home:0", Name{Raw: "home:0", Project: "home", Label: "0"}},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			assert.DeepEqual(t, Parse(c.in), c.want)
		})
	}
}

func TestShort(t *testing.T) {
	assert.Equal(t, Parse("home:2:fw").Short(), "2:fw")
	assert.Equal(t, Parse("home:2").Short(), "2")
	assert.Equal(t, Parse("home:fw").Short(), "fw")
	assert.Equal(t, Parse("slack").Short(), "slack")
}

func TestInProjectSorted(t *testing.T) {
	names := []string{"home:fw", "home:10", "web:1", "home:2:x", "home:1"}
	var got []string
	for _, n := range InProject(names, "home") {
		got = append(got, n.Raw)
	}
	assert.DeepEqual(t, got, []string{"home:1", "home:2:x", "home:10", "home:fw"})
}

func TestSlot(t *testing.T) {
	names := []string{"home:1", "home:2:fw", "web:3"}
	assert.Equal(t, Slot(names, "home", 2), "home:2:fw")
	assert.Equal(t, Slot(names, "home", 3), "home:3")
	assert.Equal(t, Slot(names, "slack", 1), "slack:1")
}

func TestProjects(t *testing.T) {
	assert.DeepEqual(t, Projects([]string{"web:1", "slack", "home:1", "home:2"}),
		[]string{"home", "main", "web"})
}

func TestFreeSlot(t *testing.T) {
	names := []string{"home:1", "home:3", "home:fw", "slack"}
	assert.Equal(t, FreeSlot(names, "home"), 2)
	assert.Equal(t, FreeSlot(names, "main"), 1)
	assert.Equal(t, FreeSlot(names, "new"), 1)
}

func TestOthers(t *testing.T) {
	assert.DeepEqual(t, Others([]string{"web:1", "slack", "home:1"}, "home"), []string{"main", "web"})
}

func TestCleanLabel(t *testing.T) {
	assert.Equal(t, CleanLabel(`  my "db"; x,y\ `), "my db xy")
}

func TestWithLabel(t *testing.T) {
	names := []string{"home:1", "home:2:fw", "home:notes", "slack"}
	cases := []struct {
		ws, label, want string
	}{
		{"home:2:fw", "db", "home:2:db"},
		{"home:2:fw", "", "home:2"},
		{"home:1", "mail", "home:1:mail"},
		{"home:notes", "todo", "home:todo"},
		{"home:notes", "", "home:3"},
		{"slack", "chat", "chat"},
		{"slack", "", "slack"},
		{"slack", "a:b", "slack"},
	}
	for _, c := range cases {
		t.Run(c.ws+"/"+c.label, func(t *testing.T) {
			assert.Equal(t, WithLabel(Parse(c.ws), c.label, names), c.want)
		})
	}
}
