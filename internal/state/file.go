package state

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// Path is keyed on the sway socket so nested sway sessions do not share
// history. XDG_RUNTIME_DIR is a tmpfs, so the file goes away with the login.
func Path(swaysock string) (string, error) {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		return "", errors.New("XDG_RUNTIME_DIR is not set")
	}
	sum := sha256.Sum256([]byte(swaysock))
	return filepath.Join(dir, "sway-ws", hex.EncodeToString(sum[:8])+".json"), nil
}

type File struct {
	path string
	lock *os.File
}

// Open takes an exclusive lock that is held until Close, so concurrent
// invocations (e.g. key repeat) apply their updates in order.
func Open(path string) (*File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		lock.Close()
		return nil, err
	}
	return &File{path: path, lock: lock}, nil
}

// Load returns an empty state if the file is missing or unreadable; history
// is a convenience and must never block navigation.
func (f *File) Load() *State {
	data, err := os.ReadFile(f.path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			warn(err)
		}
		return New()
	}
	s := New()
	if err := json.Unmarshal(data, s); err != nil {
		warn(err)
		return New()
	}
	if s.Projects == nil {
		s.Projects = map[string]*Project{}
	}
	return s
}

func (f *File) Save(s *State) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(f.path), ".state-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), f.path)
}

func (f *File) Close() error {
	return f.lock.Close()
}

func warn(err error) {
	os.Stderr.WriteString("sway-ws: ignoring state: " + err.Error() + "\n")
}
