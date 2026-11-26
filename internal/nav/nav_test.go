package nav

import (
	"testing"

	"gotest.tools/v3/assert"

	"github.com/oscarh/sway-ws/internal/state"
)

var names = []string{"web:1", "home:2:fw", "slack", "home:1"}

func TestPickItems(t *testing.T) {
	assert.DeepEqual(t, PickItems(names), []string{
		"home", "main", "web",
		"home:1", "home:2:fw", "slack", "web:1",
	})
}

func TestPickTarget(t *testing.T) {
	st := state.New()
	st.Observe("home:2:fw")
	st.Observe("web:1")

	cases := map[string]string{
		"":          "",
		"home:1":    "home:1",
		"home":      "home:2:fw",
		"slack":     "slack",
		"newws":     "web:2:newws",
		"newproj:":  "newproj:1",
		"home:":     "home:2:fw",
		"main":      "slack",
		"home:5:db": "home:5:db",
		"  web ":    "web:1",
	}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			assert.Equal(t, PickTarget(st, names, in), want)
		})
	}
}
