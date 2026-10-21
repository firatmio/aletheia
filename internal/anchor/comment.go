package anchor

import (
	"bytes"
	"path"
	"strings"
)

// commentMarkers maps file extensions to the markers that start a comment
// on a line. Files with other extensions are never reported.
var commentMarkers = map[string][]string{}

func init() {
	for _, ext := range strings.Fields(".go .js .jsx .mjs .cjs .ts .tsx .java .kt .kts .swift .c .h .cc .cpp .hpp .cs .rs .scala .dart .php") {
		commentMarkers[ext] = []string{"//", "/*"}
	}
	for _, ext := range strings.Fields(".py .sh .bash .zsh .rb .pl .r .ps1 .yml .yaml .toml") {
		commentMarkers[ext] = []string{"#"}
	}
	for _, ext := range strings.Fields(".sql .lua .hs") {
		commentMarkers[ext] = []string{"--"}
	}
	for _, ext := range strings.Fields(".html .xml .svg .vue") {
		commentMarkers[ext] = []string{"<!--"}
	}
}

// CommentOnly reports whether a contains anchor matches only inside
// comments. Such an anchor keeps holding when the code it describes
// changes, which is how a wrong claim keeps showing as verified. It is a
// heuristic (string literals are recognised by double quotes only) and only
// knows common languages; anything else reports false.
func CommentOnly(root string, a Anchor) bool {
	if a.Kind != KindContains {
		return false
	}
	markers := commentMarkers[strings.ToLower(path.Ext(a.Path))]
	if markers == nil {
		return false
	}
	full, err := resolve(root, a.Path)
	if err != nil {
		return false
	}
	data, err := readHead(full)
	if err != nil {
		return false
	}
	re, err := compileContains(a.Arg)
	if err != nil {
		return false
	}
	locs := re.FindAllIndex(data, 100)
	if len(locs) == 0 {
		return false
	}
	for _, loc := range locs {
		start := bytes.LastIndexByte(data[:loc[0]], '\n') + 1
		end := bytes.IndexByte(data[start:], '\n')
		if end < 0 {
			end = len(data) - start
		}
		line := string(data[start : start+end])
		if c := commentStart(line, markers); c < 0 || loc[0]-start < c {
			return false
		}
	}
	return true
}

// commentStart returns where a comment begins on line, or -1.
func commentStart(line string, markers []string) int {
	trimmed := strings.TrimLeft(line, " \t")
	if markers[0] == "//" && strings.HasPrefix(trimmed, "*") { // inside a /* */ block
		return len(line) - len(trimmed)
	}
	best := -1
	for _, m := range markers {
		from := 0
		for {
			i := strings.Index(line[from:], m)
			if i < 0 {
				break
			}
			i += from
			if strings.Count(line[:i], `"`)%2 == 0 { // not inside a string literal
				if best < 0 || i < best {
					best = i
				}
				break
			}
			from = i + len(m)
		}
	}
	return best
}
