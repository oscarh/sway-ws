package render

import (
	"testing"

	"gotest.tools/v3/assert"

	"github.com/oscarh/sway-ws/internal/ipc"
)

var wss = []ipc.Workspace{
	{Name: "home:2:fw", Focused: true, Visible: true},
	{Name: "home:1", Visible: true},
	{Name: "home:3", Urgent: true},
	{Name: "home:x&y"},
	{Name: "web:1", Urgent: true},
	{Name: "slack"},
}

func TestProject(t *testing.T) {
	assert.DeepEqual(t, Project(wss), Output{
		Text:    "home",
		Tooltip: "home: 1 2:fw 3 x&amp;y\nmain: slack\nweb: 1",
		Class:   []string{"project"},
	})
}

func TestOther(t *testing.T) {
	assert.DeepEqual(t, Other(wss, 1), Output{Text: "main", Tooltip: "main: slack", Class: []string{"other"}})
	assert.DeepEqual(t, Other(wss, 2), Output{Text: "web", Tooltip: "web: 1", Class: []string{"other", "urgent"}})
	assert.DeepEqual(t, Other(wss, 3), Output{})

	onOtherOutput := []ipc.Workspace{{Name: "a:1", Focused: true, Visible: true}, {Name: "b:1", Visible: true}}
	assert.DeepEqual(t, Other(onOtherOutput, 1).Class, []string{"other", "visible"})
}

func TestPosition(t *testing.T) {
	assert.DeepEqual(t, Position(wss, 1), Output{Text: "1", Tooltip: "home:1", Class: []string{"ws", "visible"}})
	assert.DeepEqual(t, Position(wss, 2), Output{Text: "2:fw", Tooltip: "home:2:fw", Class: []string{"ws", "focused"}})
	assert.DeepEqual(t, Position(wss, 3), Output{Text: "3", Tooltip: "home:3", Class: []string{"ws", "urgent"}})
	assert.DeepEqual(t, Position(wss, 4), Output{Text: "x&amp;y", Tooltip: "home:x&amp;y", Class: []string{"ws"}})
	assert.DeepEqual(t, Position(wss, 5), Output{})
	assert.DeepEqual(t, Position(nil, 1), Output{})
}

func TestMore(t *testing.T) {
	assert.DeepEqual(t, More(wss, 4), Output{})
	assert.DeepEqual(t, More(wss, 2), Output{
		Text:    "+2",
		Tooltip: "3\nx&amp;y",
		Class:   []string{"ws", "more", "urgent"},
	})
}
