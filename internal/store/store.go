package store

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// DirName is the directory that holds a project's memories.
const DirName = ".aletheia"

var (
	ErrNoStore   = errors.New("no .aletheia directory found (run `aletheia init`)")
	ErrNotFound  = errors.New("memory not found")
	ErrAmbiguous = errors.New("id prefix is ambiguous")

	prefixPattern = regexp.MustCompile(`^[a-z0-9-]{1,32}$`)
)

// Store is a directory of memory files. Root is the project root that
// anchors are resolved against; Dir is Root/.aletheia.
type Store struct {
	Root string
	Dir  string
}

// Init creates the store under root (idempotent). created reports whether
// the directory was newly made.
func Init(root string) (s *Store, created bool, err error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, false, err
	}
	s = &Store{Root: abs, Dir: filepath.Join(abs, DirName)}
	if _, statErr := os.Stat(s.Dir); statErr != nil {
		created = true
	}
	if err := os.MkdirAll(s.memDir(), 0o755); err != nil {
		return nil, false, err
	}
	gi := filepath.Join(s.Dir, ".gitignore")
	if _, err := os.Stat(gi); err != nil {
		if err := os.WriteFile(gi, []byte(".state/\n"), 0o644); err != nil {
			return nil, false, err
		}
	}
	return s, created, nil
}

// Open looks for .aletheia in start and its parents.
func Open(start string) (*Store, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return nil, err
	}
	for {
		p := filepath.Join(dir, DirName)
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			return &Store{Root: dir, Dir: p}, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, ErrNoStore
		}
		dir = parent
	}
}

func (s *Store) memDir() string     { return filepath.Join(s.Dir, "memories") }
func (s *Store) archiveDir() string { return filepath.Join(s.Dir, "archive") }

// StateDir returns a directory for machine-local, git-ignored state.
func (s *Store) StateDir(parts ...string) string {
	return filepath.Join(append([]string{s.Dir, ".state"}, parts...)...)
}

func (s *Store) path(id string) string { return filepath.Join(s.memDir(), id+".md") }

func (s *Store) read(id string) (*Memory, error) {
	data, err := os.ReadFile(s.path(id))
	if err != nil {
		return nil, err
	}
	return ParseMemory(id, data)
}

// List returns every memory plus a description of files that could not be
// parsed (one bad file never hides the rest).
func (s *Store) List() ([]*Memory, []string) {
	entries, err := os.ReadDir(s.memDir())
	if err != nil {
		return nil, nil
	}
	var out []*Memory
	var problems []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".md") {
			continue
		}
		id := strings.TrimSuffix(name, ".md")
		if !ValidID(id) {
			problems = append(problems, name+": invalid id in file name")
			continue
		}
		m, err := s.read(id)
		if err != nil {
			problems = append(problems, name+": "+err.Error())
			continue
		}
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].Created.Equal(out[j].Created) {
			return out[i].Created.Before(out[j].Created)
		}
		return out[i].ID < out[j].ID
	})
	return out, problems
}

// Get finds a memory by full id or unique prefix.
func (s *Store) Get(ref string) (*Memory, error) {
	ref = strings.ToLower(strings.TrimSpace(ref))
	if !prefixPattern.MatchString(ref) {
		return nil, fmt.Errorf("%w: %q", ErrNotFound, ref)
	}
	if ValidID(ref) {
		m, err := s.read(ref)
		if err == nil {
			return m, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	}
	entries, err := os.ReadDir(s.memDir())
	if err != nil {
		return nil, fmt.Errorf("%w: %q", ErrNotFound, ref)
	}
	var matches []string
	for _, e := range entries {
		stem := strings.TrimSuffix(e.Name(), ".md")
		if stem != e.Name() && ValidID(stem) && strings.HasPrefix(stem, ref) {
			matches = append(matches, stem)
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("%w: %q", ErrNotFound, ref)
	case 1:
		return s.read(matches[0])
	default:
		sort.Strings(matches)
		return nil, fmt.Errorf("%w: %q matches %s", ErrAmbiguous, ref, strings.Join(matches, ", "))
	}
}

// Save writes a memory atomically (temp file + rename).
func (s *Store) Save(m *Memory) error {
	if !ValidID(m.ID) {
		return fmt.Errorf("invalid memory id %q", m.ID)
	}
	if err := os.MkdirAll(s.memDir(), 0o755); err != nil {
		return err
	}
	tmp := filepath.Join(s.memDir(), "."+m.ID+".md.tmp")
	if err := os.WriteFile(tmp, m.Marshal(), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path(m.ID)); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// NewID returns an unused 6-hex-character id.
func (s *Store) NewID() (string, error) {
	for i := 0; i < 32; i++ {
		var b [3]byte
		if _, err := rand.Read(b[:]); err != nil {
			return "", err
		}
		id := hex.EncodeToString(b[:])
		_, e1 := os.Stat(s.path(id))
		_, e2 := os.Stat(filepath.Join(s.archiveDir(), id+".md"))
		if e1 != nil && e2 != nil {
			return id, nil
		}
	}
	return "", errors.New("could not allocate a memory id")
}

// Archive moves a memory out of the active set. Nothing is ever deleted.
func (s *Store) Archive(id string) error {
	if !ValidID(id) {
		return fmt.Errorf("invalid memory id %q", id)
	}
	if err := os.MkdirAll(s.archiveDir(), 0o755); err != nil {
		return err
	}
	dst := filepath.Join(s.archiveDir(), id+".md")
	if _, err := os.Stat(dst); err == nil {
		dst = filepath.Join(s.archiveDir(), fmt.Sprintf("%s-%d.md", id, time.Now().Unix()))
	}
	return os.Rename(s.path(id), dst)
}
