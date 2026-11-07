package ai

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"

	"github.com/engviniciusbs/nodegarbage/internal/rules"
	"github.com/engviniciusbs/nodegarbage/internal/tree"
	"github.com/engviniciusbs/nodegarbage/internal/volumes"
)

// Fixed limits from the spec.
const (
	DefaultBudgetTokens          = 30000
	DefaultMinUnknownBytes int64 = 500 * 1024 * 1024
	maxClassified                = 200
	maxUnknown                   = 150
	maxExtensions                = 30
	maxTopFiles                  = 50
	maxItemExts                  = 5
)

type Item struct {
	Path    string
	Size    int64
	Files   uint32
	ModTime int64
	RuleID  string
	Verdict rules.Verdict
	Exts    []string
}

type VolumeSummary struct {
	Root, FS    string
	Total, Free uint64
}

type Payload struct {
	Volumes    []VolumeSummary
	Classified []Item
	Unknown    []Item
	Extensions []tree.ExtStat
	TopFiles   []tree.FileInfo
}

// PathMap maps an anonymized path (rules.NormalizePath form) to the real path.
type PathMap map[string]string

type ScanData struct {
	Volume  volumes.Volume
	Tree    *tree.Tree
	Classes *rules.Classifications
}

// PayloadOptions: zero values mean the defaults.
type PayloadOptions struct {
	BudgetTokens    int
	MinUnknownBytes int64
}

// EstimateTokens is len(b)/4 rounded up.
func EstimateTokens(b []byte) int { return (len(b) + 3) / 4 }

type rawItem struct {
	Item
	real string
}

// BuildPayload assembles the anonymized payload and trims it to the budget.
func BuildPayload(scans []ScanData, anon *Anonymizer, opts PayloadOptions) (Payload, PathMap) {
	if opts.BudgetTokens <= 0 {
		opts.BudgetTokens = DefaultBudgetTokens
	}
	if opts.MinUnknownBytes <= 0 {
		opts.MinUnknownBytes = DefaultMinUnknownBytes
	}
	var p Payload
	var classified, unknown []rawItem
	var top []tree.FileInfo
	exts := map[string]*tree.ExtStat{}

	for _, s := range scans {
		p.Volumes = append(p.Volumes, VolumeSummary{Root: s.Volume.Root, FS: s.Volume.FS, Total: s.Volume.Total, Free: s.Volume.Free})
		if s.Tree == nil || s.Classes == nil || len(s.Tree.Nodes) == 0 {
			continue
		}
		c, u := collect(s, opts.MinUnknownBytes)
		classified = append(classified, c...)
		unknown = append(unknown, u...)
		top = append(top, s.Tree.TopFiles...)
		for _, e := range s.Tree.Extensions {
			x := exts[e.Ext]
			if x == nil {
				x = &tree.ExtStat{Ext: e.Ext}
				exts[e.Ext] = x
			}
			x.Count += e.Count
			x.Bytes += e.Bytes
		}
	}
	sortItems(classified)
	sortItems(unknown)
	sort.SliceStable(top, func(i, j int) bool {
		if top[i].Size != top[j].Size {
			return top[i].Size > top[j].Size
		}
		return top[i].Path < top[j].Path
	})
	for _, e := range exts {
		// Extensions come from real file names: anonymize, and drop any that still leak.
		if name := anon.Apply(e.Ext); !anon.Leaks(name) {
			p.Extensions = append(p.Extensions, tree.ExtStat{Ext: name, Count: e.Count, Bytes: e.Bytes})
		}
	}
	sort.Slice(p.Extensions, func(i, j int) bool {
		a, b := p.Extensions[i], p.Extensions[j]
		if a.Bytes != b.Bytes {
			return a.Bytes > b.Bytes
		}
		return a.Ext < b.Ext
	})
	if len(p.Extensions) > maxExtensions {
		p.Extensions = p.Extensions[:maxExtensions]
	}

	// Anonymize in size order; on collision the first (largest) wins.
	pm := PathMap{}
	claim := func(real string) (string, bool) {
		a := anon.Apply(real)
		if anon.Leaks(a) {
			return "", false
		}
		k := rules.NormalizePath(a)
		if _, dup := pm[k]; dup {
			return "", false
		}
		pm[k] = real
		return a, true
	}
	for _, it := range classified {
		if len(p.Classified) == maxClassified {
			break
		}
		if a, ok := claim(it.real); ok {
			it.Path = a
			p.Classified = append(p.Classified, it.Item)
		}
	}
	for _, it := range unknown {
		if len(p.Unknown) == maxUnknown {
			break
		}
		if a, ok := claim(it.real); ok {
			it.Path = a
			p.Unknown = append(p.Unknown, it.Item)
		}
	}
	for _, f := range top {
		if len(p.TopFiles) == maxTopFiles {
			break
		}
		if a, ok := claim(f.Path); ok {
			p.TopFiles = append(p.TopFiles, tree.FileInfo{Path: a, Size: f.Size, ModTime: f.ModTime})
		}
	}

	// Per-item extensions come from real file names: anonymize and guard them too.
	for i := range p.Unknown {
		var ex []string
		for _, e := range p.Unknown[i].Exts {
			if e = anon.Apply(e); !anon.Leaks(e) {
				ex = append(ex, e)
			}
		}
		p.Unknown[i].Exts = ex
	}

	trim(&p, opts.BudgetTokens)

	// Keep only the paths that survived trimming.
	final := PathMap{}
	keep := func(anonPath string) {
		k := rules.NormalizePath(anonPath)
		final[k] = pm[k]
	}
	for _, it := range p.Classified {
		keep(it.Path)
	}
	for _, it := range p.Unknown {
		keep(it.Path)
	}
	for _, f := range p.TopFiles {
		keep(f.Path)
	}
	return p, final
}

func sortItems(items []rawItem) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Size != items[j].Size {
			return items[i].Size > items[j].Size
		}
		return items[i].real < items[j].real
	})
}

// collect returns the maximal classified nodes and the unknown candidates of
// one scan. An unknown node's size is its unclassified bytes: its own files
// plus those of unknown descendants, excluding classified subtrees. Children
// always have a higher id than their parent, so one reverse pass is post-order.
func collect(s ScanData, minUnknown int64) (classified, unknown []rawItem) {
	t := s.Tree
	n := len(t.Nodes)
	cls := make([]rules.Classification, n)
	for i := range t.Nodes {
		cls[i] = s.Classes.Get(t, tree.NodeID(i))
	}
	uncl := make([]int64, n)
	for i := n - 1; i >= 0; i-- {
		if cls[i].Source != rules.SourceUnknown {
			continue
		}
		sum := t.Nodes[i].OwnSize
		for _, ch := range t.Nodes[i].Children {
			sum += uncl[ch]
		}
		uncl[i] = sum
	}

	var needExts []int
	for i := range t.Nodes {
		id := tree.NodeID(i)
		nd := &t.Nodes[i]
		if id == t.Root || nd.Flags&tree.FlagLink != 0 {
			continue
		}
		c := cls[i]
		if c.Source == rules.SourceUnknown {
			if uncl[i] < minUnknown {
				continue
			}
			unknown = append(unknown, rawItem{Item: Item{Size: uncl[i], Files: nd.TotalFiles, ModTime: nd.TreeLatest}, real: t.Path(id)})
			needExts = append(needExts, len(unknown)-1)
			continue
		}
		// Only rule-classified items are listed (spec §5); a plain protected
		// node (no rule matched it) is neither classified nor unknown.
		if c.RuleID == "" && c.Source != rules.SourceRule && c.Source != rules.SourceAI {
			continue
		}
		// Maximal: the parent (unless it is the volume root) does not carry the same classification.
		if nd.Parent != t.Root && nd.Parent != tree.NoNode && cls[nd.Parent] == c {
			continue
		}
		classified = append(classified, rawItem{Item: Item{Size: nd.TotalSize, Files: nd.TotalFiles, ModTime: nd.TreeLatest, RuleID: c.RuleID, Verdict: c.Verdict}, real: t.Path(id)})
	}

	if len(needExts) > 0 {
		norm := make([]string, len(t.TopFiles))
		for i, f := range t.TopFiles {
			norm[i] = rules.NormalizePath(f.Path)
		}
		for _, idx := range needExts {
			unknown[idx].Exts = topExts(t.TopFiles, norm, unknown[idx].real)
		}
	}
	return classified, unknown
}

// topExts: the largest extensions among the top files inside dir (an
// approximation: only the scan's top-N files are known per extension).
func topExts(files []tree.FileInfo, norm []string, dir string) []string {
	prefix := rules.NormalizePath(dir)
	if !strings.HasSuffix(prefix, `\`) {
		prefix += `\`
	}
	bytesBy := map[string]int64{}
	for i, f := range files {
		if strings.HasPrefix(norm[i], prefix) {
			bytesBy[extOf(f.Path)] += f.Size
		}
	}
	if len(bytesBy) == 0 {
		return nil
	}
	out := make([]string, 0, len(bytesBy))
	for e := range bytesBy {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		if bytesBy[out[i]] != bytesBy[out[j]] {
			return bytesBy[out[i]] > bytesBy[out[j]]
		}
		return out[i] < out[j]
	})
	if len(out) > maxItemExts {
		out = out[:maxItemExts]
	}
	return out
}

func extOf(path string) string {
	name := path[strings.LastIndex(path, `\`)+1:]
	i := strings.LastIndex(name, ".")
	if i <= 0 {
		return ""
	}
	return strings.ToLower(name[i:])
}

// ---- compact encoding ----

type table struct {
	Cols []string `json:"cols"`
	Rows [][]any  `json:"rows"`
}

type compact struct {
	Volumes    table `json:"volumes"`
	Classified table `json:"classified"`
	Unknown    table `json:"unknown"`
	Extensions table `json:"extensions"`
	TopFiles   table `json:"topFiles"`
}

func marshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false) // keep "<user>" readable and short
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

func volumeRow(v VolumeSummary) []any { return []any{v.Root, v.FS, v.Total, v.Free} }
func classifiedRow(i Item) []any {
	return []any{i.Path, i.Size, i.Files, i.ModTime, i.RuleID, string(i.Verdict)}
}
func unknownRow(i Item) []any {
	e := i.Exts
	if e == nil {
		e = []string{}
	}
	return []any{i.Path, i.Size, i.Files, i.ModTime, e}
}
func extRow(e tree.ExtStat) []any   { return []any{e.Ext, e.Count, e.Bytes} }
func fileRow(f tree.FileInfo) []any { return []any{f.Path, f.Size, f.ModTime} }

func rowsOf[T any](xs []T, f func(T) []any) [][]any {
	rows := make([][]any, len(xs)) // never nil: encodes as []
	for i, x := range xs {
		rows[i] = f(x)
	}
	return rows
}

func (p Payload) build() compact {
	return compact{
		Volumes:    table{[]string{"root", "fs", "total", "free"}, rowsOf(p.Volumes, volumeRow)},
		Classified: table{[]string{"path", "size", "files", "mtime", "rule", "verdict"}, rowsOf(p.Classified, classifiedRow)},
		Unknown:    table{[]string{"path", "size", "files", "mtime", "exts"}, rowsOf(p.Unknown, unknownRow)},
		Extensions: table{[]string{"ext", "count", "bytes"}, rowsOf(p.Extensions, extRow)},
		TopFiles:   table{[]string{"path", "size", "mtime"}, rowsOf(p.TopFiles, fileRow)},
	}
}

// Compact encodes the payload deterministically; every list is
// {"cols":[...],"rows":[[...]]}.
func (p Payload) Compact() ([]byte, error) { return marshal(p.build()) }

// ---- budget ----

func rowLens[T any](xs []T, f func(T) []any) ([]int, error) {
	out := make([]int, len(xs))
	for i, x := range xs {
		b, err := marshal(f(x))
		if err != nil {
			return nil, err
		}
		out[i] = len(b)
	}
	return out, nil
}

// trim drops the smallest items round-robin from Classified, Unknown and
// TopFiles (keeping the largest of each), then extensions, until the encoded
// size fits the budget. Lists are sorted desc, so the smallest is the last.
// Sizes are tracked incrementally from per-row lengths (exact: a list of n
// rows adds sum(len)+n-1 over its empty encoding), avoiding re-marshalling.
func trim(p *Payload, budget int) {
	empty := *p
	empty.Classified, empty.Unknown, empty.Extensions, empty.TopFiles = nil, nil, nil, nil
	base, err := empty.Compact()
	if err != nil {
		return
	}
	cl, e1 := rowLens(p.Classified, classifiedRow)
	ul, e2 := rowLens(p.Unknown, unknownRow)
	tl, e3 := rowLens(p.TopFiles, fileRow)
	el, e4 := rowLens(p.Extensions, extRow)
	if e1 != nil || e2 != nil || e3 != nil || e4 != nil {
		return
	}
	total := len(base)
	for _, l := range [][]int{cl, ul, tl, el} {
		for i, n := range l {
			total += n
			if i > 0 {
				total++
			}
		}
	}
	over := func() bool { return (total+3)/4 > budget }
	drop := func(l *[]int) {
		n := len(*l)
		total -= (*l)[n-1]
		if n > 1 {
			total--
		}
		*l = (*l)[:n-1]
	}
	for over() {
		progressed := false
		if len(cl) > 1 && over() {
			drop(&cl)
			p.Classified = p.Classified[:len(cl)]
			progressed = true
		}
		if len(ul) > 1 && over() {
			drop(&ul)
			p.Unknown = p.Unknown[:len(ul)]
			progressed = true
		}
		if len(tl) > 1 && over() {
			drop(&tl)
			p.TopFiles = p.TopFiles[:len(tl)]
			progressed = true
		}
		if !progressed {
			if len(el) == 0 {
				return
			}
			drop(&el)
			p.Extensions = p.Extensions[:len(el)]
		}
	}
}
