package app

import (
	"os"
	"sort"
	"strings"
	"time"

	"github.com/engviniciusbs/nodegarbage/internal/rules"
	"github.com/engviniciusbs/nodegarbage/internal/tree"
)

const maxListFiles = 5000

func verdictStr(v rules.Verdict) string {
	if v == "" {
		return "unknown"
	}
	return string(v)
}

// ListVolumes lista os volumes com última varredura e lixo estimado.
func (a *App) ListVolumes() ([]VolumeDTO, error) {
	if a.deps.Volumes == nil {
		return nil, errNotSupported
	}
	vols, err := a.deps.Volumes()
	if err != nil {
		return nil, err
	}
	var latest map[string]time.Time
	if a.deps.Store != nil {
		latest, _ = a.deps.Store.LatestScans(a.baseCtx())
	}
	out := make([]VolumeDTO, 0, len(vols))
	for _, v := range vols {
		d := VolumeDTO{Root: v.Root, Label: v.Label, FS: v.FS, Type: string(v.Type),
			Total: int64(v.Total), Free: int64(v.Free), HasRecycleBin: v.HasRecycleBin}
		if s, err := a.session(v.Root); err == nil {
			at := s.scannedAt
			d.LastScan = &at
			d.EstimatedJunk = a.summarize(s).SafeBytes
		} else {
			for k, at := range latest {
				if volKey(k) == volKey(v.Root) {
					at := at
					d.LastScan = &at
				}
			}
		}
		out = append(out, d)
	}
	return out, nil
}

func nodeID(s *session, id int32) (tree.NodeID, bool) {
	if len(s.tree.Nodes) == 0 {
		return tree.NoNode, false
	}
	if id == -1 {
		return s.tree.Root, true
	}
	if id < 0 || int(id) >= len(s.tree.Nodes) {
		return tree.NoNode, false
	}
	return tree.NodeID(id), true
}

func (a *App) nodeDTO(s *session, id tree.NodeID) NodeDTO {
	n := s.tree.Nodes[id]
	cls := s.classes.Get(s.tree, id)
	d := NodeDTO{
		ID: int32(id), Name: n.Name, Path: s.tree.Path(id), Size: n.TotalSize, Files: n.TotalFiles,
		LatestMod: n.TreeLatest, HasChildren: len(n.Children) > 0,
		Link: n.Flags&tree.FlagLink != 0, Cloud: n.Flags&tree.FlagCloud != 0, Denied: n.Flags&tree.FlagDenied != 0,
		Inactive: cls.Inactive, Verdict: verdictStr(cls.Verdict), Category: string(cls.Category),
		Source: string(cls.Source), RuleID: cls.RuleID, Action: string(cls.Action),
	}
	if d.Source == "" {
		d.Source = string(rules.SourceUnknown)
	}
	if a.deps.Classifier != nil && cls.RuleID != "" {
		if r, ok := a.deps.Classifier.Rule(cls.RuleID); ok {
			d.Explain, d.Regenerable, d.Command = r.Explain, r.Regenerable, r.Command
		}
	}
	if op, ok := s.aiOpinion[id]; ok {
		d.AI = &AIOpinionDTO{Verdict: string(op.Verdict), Reason: op.Reason, Confidence: op.Confidence}
	}
	return d
}

// GetChildren devolve os filhos do nó (id -1 = raiz) por tamanho decrescente.
func (a *App) GetChildren(volume string, id int32) ([]NodeDTO, error) {
	s, err := a.session(volume)
	if err != nil {
		return nil, err
	}
	nid, ok := nodeID(s, id)
	if !ok {
		return nil, errNotScanned
	}
	kids := s.tree.SortedChildren(nid)
	out := make([]NodeDTO, 0, len(kids))
	for _, k := range kids {
		out = append(out, a.nodeDTO(s, k))
	}
	return out, nil
}

func extendedPath(p string) string {
	if strings.HasPrefix(p, `\\`) || len(p) < 240 {
		return p
	}
	return `\\?\` + p
}

func (a *App) fileVerdict(path string, mod time.Time, dir rules.Classification) (string, string) {
	if a.deps.Classifier != nil {
		if c, ok := a.deps.Classifier.ClassifyFile(path, mod, a.deps.Now()); ok {
			return verdictStr(c.Verdict), c.RuleID
		}
	}
	return verdictStr(dir.Verdict), dir.RuleID
}

// ListFiles lista os arquivos diretos de um diretório (leitura do disco).
func (a *App) ListFiles(volume string, id int32) ([]FileDTO, error) {
	s, err := a.session(volume)
	if err != nil {
		return nil, err
	}
	nid, ok := nodeID(s, id)
	if !ok {
		return nil, errNotScanned
	}
	dir := s.tree.Path(nid)
	ents, err := os.ReadDir(extendedPath(dir))
	if err != nil {
		return nil, err
	}
	dcls := s.classes.Get(s.tree, nid)
	out := []FileDTO{}
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		p := strings.TrimRight(dir, `\`) + `\` + e.Name()
		v, r := a.fileVerdict(p, info.ModTime(), dcls)
		out = append(out, FileDTO{Name: e.Name(), Path: p, Size: info.Size(), ModTime: info.ModTime().Unix(), Verdict: v, RuleID: r})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Size > out[j].Size })
	if len(out) > maxListFiles {
		out = out[:maxListFiles]
	}
	return out, nil
}

// GetTopFiles devolve os maiores arquivos do volume.
func (a *App) GetTopFiles(volume string) ([]FileDTO, error) {
	s, err := a.session(volume)
	if err != nil {
		return nil, err
	}
	out := make([]FileDTO, 0, len(s.tree.TopFiles))
	for _, f := range s.tree.TopFiles {
		name, dirPath := f.Path, ""
		if i := strings.LastIndexByte(f.Path, '\\'); i >= 0 {
			name, dirPath = f.Path[i+1:], f.Path[:i+1]
		}
		var dcls rules.Classification
		if did, ok := s.tree.Find(dirPath); ok {
			dcls = s.classes.Get(s.tree, did)
		}
		v, r := a.fileVerdict(f.Path, time.Unix(f.ModTime, 0), dcls)
		out = append(out, FileDTO{Name: name, Path: f.Path, Size: f.Size, ModTime: f.ModTime, Verdict: v, RuleID: r})
	}
	return out, nil
}

// GetExtensions devolve o uso de espaço por extensão.
func (a *App) GetExtensions(volume string) ([]ExtDTO, error) {
	s, err := a.session(volume)
	if err != nil {
		return nil, err
	}
	out := make([]ExtDTO, 0, len(s.tree.Extensions))
	for _, e := range s.tree.Extensions {
		out = append(out, ExtDTO{Ext: e.Ext, Count: e.Count, Bytes: e.Bytes})
	}
	return out, nil
}

// GetSummary soma os nós classificados de topo (sem contar duas vezes).
func (a *App) GetSummary(volume string) (SummaryDTO, error) {
	s, err := a.session(volume)
	if err != nil {
		return SummaryDTO{}, err
	}
	sum := a.summarize(s)
	sum.Volume = volume
	return sum, nil
}

func (a *App) summarize(s *session) SummaryDTO {
	sum := SummaryDTO{ByCategory: []CategoryDTO{}}
	cats := map[string]*CategoryDTO{}
	t := s.tree
	for i := range t.Nodes {
		id := tree.NodeID(i)
		c := s.classes.Get(t, id)
		if c.Verdict == "" || c.Category == "" {
			continue
		}
		switch c.Source {
		case rules.SourceRule, rules.SourceAI:
		case rules.SourceProtected:
			if c.RuleID == "" {
				continue // plain protected areas are not reported
			}
		default:
			continue
		}
		if p := t.Nodes[i].Parent; p != tree.NoNode {
			pc := s.classes.Get(t, p)
			if pc.Source == c.Source && pc.RuleID == c.RuleID && pc.Verdict == c.Verdict {
				continue // herdado do pai
			}
		}
		size := t.Nodes[i].TotalSize
		switch c.Verdict {
		case rules.Safe:
			sum.SafeBytes += size
		case rules.Caution:
			sum.CautionBytes += size
		case rules.Keep:
			sum.KeepBytes += size
		}
		cd := cats[string(c.Category)]
		if cd == nil {
			cd = &CategoryDTO{Category: string(c.Category)}
			cats[string(c.Category)] = cd
		}
		cd.Bytes += size
		cd.Count++
	}
	for _, c := range cats {
		sum.ByCategory = append(sum.ByCategory, *c)
	}
	sort.Slice(sum.ByCategory, func(i, j int) bool {
		if sum.ByCategory[i].Bytes != sum.ByCategory[j].Bytes {
			return sum.ByCategory[i].Bytes > sum.ByCategory[j].Bytes
		}
		return sum.ByCategory[i].Category < sum.ByCategory[j].Category
	})
	return sum
}
