// Package anchor ties a memory claim to checkable evidence in the project.
//
// Anchors are plain strings stored in a memory's frontmatter:
//
//	file:go.mod@<sha256>        the file's content hash is unchanged
//	contains:go.mod::go 1\.23   the file still matches this regular expression
//	                            (runs of spaces/tabs match any run of spaces/tabs)
//	exists:src/auth             the path still exists
//	symbol:store.go::Store.Get  the file still declares this function, method,
//	                            type, var or const
//
// Verification only reads files inside the project root. It never executes
// anything, so a poisoned memory file cannot run code on the machine.
package anchor

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	maxHashBytes = 32 << 20 // refuse to hash files larger than this
	maxScanBytes = 4 << 20  // `contains` only scans this much of a file
)

// Kind is the type of an anchor.
type Kind string

// Supported anchor kinds.
const (
	KindFile     Kind = "file"
	KindContains Kind = "contains"
	KindExists   Kind = "exists"
	KindSymbol   Kind = "symbol"
)

// ErrFormat is returned for malformed anchors.
var ErrFormat = errors.New("invalid anchor")

var errMissing = errors.New("missing")

// Anchor is a parsed anchor. Arg is the content hash for file anchors, the
// regular expression for contains anchors and the name for symbol anchors.
type Anchor struct {
	Kind Kind
	Path string
	Arg  string
}

// Result is the outcome of checking one anchor.
type Result struct {
	OK      bool
	Detail  string // short reason when !OK, e.g. "go.mod: changed"
	Line    int    // first line of the match or declaration (contains, symbol); 0 if unknown
	EndLine int    // last line of it
}

// String returns the canonical text form.
func (a Anchor) String() string {
	switch a.Kind {
	case KindFile:
		if a.Arg == "" {
			return "file:" + a.Path
		}
		return "file:" + a.Path + "@" + a.Arg
	case KindContains:
		return "contains:" + a.Path + "::" + a.Arg
	case KindSymbol:
		return "symbol:" + a.Path + "::" + a.Arg
	default:
		return "exists:" + a.Path
	}
}

func isSHA256(s string) bool {
	if len(s) != 64 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

func cleanRel(p string) (string, error) {
	p = filepath.ToSlash(strings.TrimSpace(p))
	if p == "" {
		return "", fmt.Errorf("%w: empty path", ErrFormat)
	}
	if path.IsAbs(p) || (len(p) >= 2 && p[1] == ':') {
		return "", fmt.Errorf("%w: path must be relative to the project root", ErrFormat)
	}
	c := path.Clean(p)
	if c == "." || c == ".." || strings.HasPrefix(c, "../") {
		return "", fmt.Errorf("%w: path escapes the project root", ErrFormat)
	}
	return c, nil
}

// Parse parses the text form of an anchor. A file anchor without a hash is
// accepted (it is "unsealed"; see Make).
func Parse(raw string) (Anchor, error) {
	kind, rest, ok := strings.Cut(strings.TrimSpace(raw), ":")
	if !ok || rest == "" {
		return Anchor{}, fmt.Errorf("%w: %q (want kind:path; kinds: file, contains, exists, symbol)", ErrFormat, raw)
	}
	switch Kind(kind) {
	case KindFile:
		p, h := rest, ""
		if i := strings.LastIndex(rest, "@"); i > 0 && isSHA256(rest[i+1:]) {
			p, h = rest[:i], rest[i+1:]
		}
		cp, err := cleanRel(p)
		if err != nil {
			return Anchor{}, err
		}
		return Anchor{Kind: KindFile, Path: cp, Arg: h}, nil
	case KindContains:
		p, re, ok := strings.Cut(rest, "::")
		if !ok || strings.TrimSpace(p) == "" || re == "" {
			return Anchor{}, fmt.Errorf("%w: contains needs path::regex", ErrFormat)
		}
		if _, err := regexp.Compile("(?m)" + re); err != nil {
			return Anchor{}, fmt.Errorf("%w: bad regex: %v", ErrFormat, err)
		}
		cp, err := cleanRel(p)
		if err != nil {
			return Anchor{}, err
		}
		return Anchor{Kind: KindContains, Path: cp, Arg: re}, nil
	case KindExists:
		cp, err := cleanRel(rest)
		if err != nil {
			return Anchor{}, err
		}
		return Anchor{Kind: KindExists, Path: cp}, nil
	case KindSymbol:
		p, name, ok := strings.Cut(rest, "::")
		if !ok || !symbolName.MatchString(name) {
			return Anchor{}, fmt.Errorf("%w: symbol needs path::Name or path::Type.Method", ErrFormat)
		}
		cp, err := cleanRel(p)
		if err != nil {
			return Anchor{}, err
		}
		return Anchor{Kind: KindSymbol, Path: cp, Arg: name}, nil
	}
	return Anchor{}, fmt.Errorf("%w: unknown kind %q (want file, contains, exists, symbol)", ErrFormat, kind)
}

// resolve maps a project-relative path to a real path, refusing anything
// that leaves the project root (including via symlinks).
func resolve(root, rel string) (string, error) {
	full := filepath.Join(root, filepath.FromSlash(rel))
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	realFull, err := filepath.EvalSymlinks(full)
	if err != nil {
		if os.IsNotExist(err) {
			return "", errMissing
		}
		return "", err
	}
	r, err := filepath.Rel(realRoot, realFull)
	if err != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
		return "", errors.New("resolves outside the project root")
	}
	return realFull, nil
}

// normalizeText converts CRLF line endings to LF in text files so that the
// same commit hashes identically on Windows (core.autocrlf) and Unix. Files
// with a NUL byte in their first 8 KiB are treated as binary and left alone.
func normalizeText(data []byte) []byte {
	head := data
	if len(head) > 8192 {
		head = head[:8192]
	}
	if bytes.IndexByte(head, 0) >= 0 {
		return data
	}
	return bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
}

func hashFile(full string) (string, error) {
	f, err := os.Open(full)
	if err != nil {
		return "", err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return "", err
	}
	if st.IsDir() {
		return "", errors.New("is a directory")
	}
	if st.Size() > maxHashBytes {
		return "", errors.New("file too large to hash")
	}
	data, err := io.ReadAll(io.LimitReader(f, maxHashBytes+1))
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(normalizeText(data))
	return hex.EncodeToString(sum[:]), nil
}

func readHead(full string) ([]byte, error) {
	f, err := os.Open(full)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if st.IsDir() {
		return nil, errors.New("is a directory")
	}
	data, err := io.ReadAll(io.LimitReader(f, maxScanBytes))
	if err != nil {
		return nil, err
	}
	return normalizeText(data), nil
}

// compileContains compiles a contains pattern in multi-line mode, with
// runs of spaces and tabs relaxed (see relaxSpaces).
func compileContains(pattern string) (*regexp.Regexp, error) {
	return regexp.Compile("(?m)" + relaxSpaces(pattern))
}

// relaxSpaces lets every run of literal spaces and tabs in a pattern match
// any run of spaces and tabs, so realigning code (gofmt, prettier) does not
// stale an anchor. Runs that are escaped, inside a character class, or
// followed by a quantifier keep their meaning.
func relaxSpaces(pattern string) string {
	var b strings.Builder
	inClass := false
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch {
		case c == '\\' && i+1 < len(pattern):
			b.WriteString(pattern[i : i+2])
			i++
		case inClass:
			if c == ']' {
				inClass = false
			}
			b.WriteByte(c)
		case c == '[':
			inClass = true
			b.WriteByte(c)
			if i+1 < len(pattern) && pattern[i+1] == '^' {
				b.WriteByte('^')
				i++
			}
			if i+1 < len(pattern) && pattern[i+1] == ']' { // a leading ] is literal
				b.WriteByte(']')
				i++
			}
		case c == ' ' || c == '\t':
			j := i
			for j < len(pattern) && (pattern[j] == ' ' || pattern[j] == '\t') {
				j++
			}
			if j < len(pattern) && isQuantifier(pattern[j:]) {
				b.WriteString(pattern[i:j])
			} else {
				b.WriteString(`[ \t]+`)
			}
			i = j - 1
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

func isQuantifier(s string) bool {
	switch s[0] {
	case '*', '+', '?':
		return true
	case '{':
		return len(s) > 1 && s[1] >= '0' && s[1] <= '9'
	}
	return false
}

// Check verifies one anchor against the files under root.
func Check(root string, a Anchor) Result {
	full, err := resolve(root, a.Path)
	if err != nil {
		if errors.Is(err, errMissing) {
			return Result{Detail: a.Path + ": missing"}
		}
		return Result{Detail: a.Path + ": " + err.Error()}
	}
	switch a.Kind {
	case KindExists:
		return Result{OK: true}
	case KindFile:
		if a.Arg == "" {
			return Result{Detail: a.Path + ": unsealed anchor (no hash)"}
		}
		sum, err := hashFile(full)
		if err != nil {
			return Result{Detail: a.Path + ": " + err.Error()}
		}
		if sum != a.Arg {
			return Result{Detail: a.Path + ": changed"}
		}
		return Result{OK: true}
	case KindContains:
		data, err := readHead(full)
		if err != nil {
			return Result{Detail: a.Path + ": " + err.Error()}
		}
		re, err := compileContains(a.Arg)
		if err != nil {
			return Result{Detail: a.Path + ": bad regex"}
		}
		loc := re.FindIndex(data)
		if loc == nil {
			return Result{Detail: a.Path + ": pattern not found"}
		}
		start, end := lineOf(data, loc[0]), lineOf(data, max(loc[0], loc[1]-1))
		return Result{OK: true, Line: start, EndLine: end}
	case KindSymbol:
		data, err := readHead(full)
		if err != nil {
			return Result{Detail: a.Path + ": " + err.Error()}
		}
		start, end, ok := findSymbol(a.Path, data, a.Arg)
		if !ok {
			return Result{Detail: a.Path + ": " + a.Arg + " not found"}
		}
		return Result{OK: true, Line: start, EndLine: end}
	}
	return Result{Detail: "unknown anchor kind"}
}

// Make parses a user-supplied spec, seals file anchors with the current
// hash, and refuses anchors that do not hold right now. This is what stops
// an agent from recording a claim its own evidence contradicts.
func Make(root, spec string) (Anchor, error) {
	a, err := Parse(spec)
	if err != nil {
		return Anchor{}, err
	}
	if a.Kind == KindFile && a.Arg == "" {
		full, err := resolve(root, a.Path)
		if err != nil {
			return Anchor{}, fmt.Errorf("%s: %w", a.Path, err)
		}
		sum, err := hashFile(full)
		if err != nil {
			return Anchor{}, fmt.Errorf("%s: %w", a.Path, err)
		}
		a.Arg = sum
	}
	if r := Check(root, a); !r.OK {
		return Anchor{}, fmt.Errorf("anchor does not hold right now: %s", r.Detail)
	}
	return a, nil
}

// Reseal re-hashes a file anchor after a human or agent has re-checked the
// claim. Other kinds must still hold as written.
func Reseal(root string, a Anchor) (Anchor, error) {
	if a.Kind != KindFile {
		if r := Check(root, a); !r.OK {
			return a, errors.New(r.Detail)
		}
		return a, nil
	}
	full, err := resolve(root, a.Path)
	if err != nil {
		return a, fmt.Errorf("%s: %w", a.Path, err)
	}
	sum, err := hashFile(full)
	if err != nil {
		return a, fmt.Errorf("%s: %w", a.Path, err)
	}
	a.Arg = sum
	return a, nil
}
