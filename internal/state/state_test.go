package state

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"gotest.tools/v3/assert"
)

var names = []string{"home:1", "home:2:fw", "home:3", "web:1", "web:2"}

func TestObserveTracksPerProject(t *testing.T) {
	s := New()
	s.Observe("home:1")
	s.Observe("home:2:fw")
	s.Observe("web:2")
	s.Observe("web:1")

	assert.DeepEqual(t, *s.Projects["home"], Project{Last: "home:2:fw", Prev: "home:1"})
	assert.DeepEqual(t, *s.Projects["web"], Project{Last: "web:1", Prev: "web:2"})
	assert.Equal(t, s.Current, "web")
	assert.Equal(t, s.ProjectPrev, "home")
}

func TestObserveSameWorkspaceKeepsPrev(t *testing.T) {
	s := New()
	s.Observe("home:1")
	s.Observe("home:2:fw")
	s.Observe("home:2:fw")
	assert.DeepEqual(t, *s.Projects["home"], Project{Last: "home:2:fw", Prev: "home:1"})
}

func TestProjectToggleReturnsToLastWorkspace(t *testing.T) {
	s := New()
	s.Observe("home:3")
	s.Observe("web:2")

	ws, ok := s.ProjectToggleTarget(names)
	assert.Assert(t, ok)
	assert.Equal(t, ws, "home:3")

	s.Observe(ws)
	ws, ok = s.ProjectToggleTarget(names)
	assert.Assert(t, ok)
	assert.Equal(t, ws, "web:2")
}

func TestToggleStaysInProject(t *testing.T) {
	s := New()
	s.Observe("home:1")
	s.Observe("web:1")
	_, ok := s.ToggleTarget(names)
	assert.Assert(t, !ok)

	s.Observe("web:2")
	ws, ok := s.ToggleTarget(names)
	assert.Assert(t, ok)
	assert.Equal(t, ws, "web:1")
}

func TestProjectTargetFallsBack(t *testing.T) {
	s := New()
	s.Observe("home:9")

	ws, ok := s.ProjectTarget("home", names)
	assert.Assert(t, ok)
	assert.Equal(t, ws, "home:1")

	_, ok = s.ProjectTarget("gone", names)
	assert.Assert(t, !ok)
}

func TestRenameIntoOtherProjectIsIgnored(t *testing.T) {
	s := New()
	s.Observe("home:3")
	s.Rename("home:3", "web:3")
	ws, ok := s.ProjectTarget("home", append(names, "web:3"))
	assert.Assert(t, ok)
	assert.Equal(t, ws, "home:1")
}

func TestPrune(t *testing.T) {
	s := New()
	s.Observe("gone:1")
	s.Observe("home:1")
	s.Prune(names)
	_, ok := s.Projects["gone"]
	assert.Assert(t, !ok)
	_, ok = s.Projects["home"]
	assert.Assert(t, ok)
}

func TestFileRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "state.json")
	f, err := Open(path)
	assert.NilError(t, err)
	s := f.Load()
	s.Observe("home:1")
	assert.NilError(t, f.Save(s))
	assert.NilError(t, f.Close())

	f, err = Open(path)
	assert.NilError(t, err)
	defer f.Close()
	assert.DeepEqual(t, f.Load(), s)
}

func TestFileCorruptIsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	assert.NilError(t, os.WriteFile(path, []byte("{nope"), 0o600))
	f, err := Open(path)
	assert.NilError(t, err)
	defer f.Close()
	assert.DeepEqual(t, f.Load(), New())
}

func TestSnapshotAndRestore(t *testing.T) {
	s := New()
	s.Observe("home:1")
	s.Snapshot([]string{"home:1", "home:4"})
	s.Observe("web:1")
	s.Snapshot(nil)
	assert.DeepEqual(t, s.Projects["home"].Visible, []string{"home:1", "home:4"})

	all := slices.Concat(names, []string{"home:4"})
	assert.DeepEqual(t, s.Restore("home", "home:1", all, []string{"web:1"}), []string{"home:4"})
	assert.DeepEqual(t, s.Restore("home", "home:4", all, []string{"web:1"}), []string{"home:1"})
	// Already on screen, or gone.
	assert.DeepEqual(t, s.Restore("home", "home:1", all, []string{"home:4"}), []string(nil))
	assert.DeepEqual(t, s.Restore("home", "home:1", names, nil), []string(nil))
	assert.DeepEqual(t, s.Restore("nope", "nope:1", all, nil), []string(nil))
}

func TestRenameUpdatesVisible(t *testing.T) {
	s := New()
	s.Observe("home:1")
	s.Snapshot([]string{"home:1", "home:2:fw"})
	s.Rename("home:2:fw", "home:2:db")
	assert.DeepEqual(t, s.Projects["home"].Visible, []string{"home:1", "home:2:db"})
}
