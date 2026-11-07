package store

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/engviniciusbs/nodegarbage/internal/tree"
)

func openTemp(t *testing.T) (*Store, string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "sub", "nodegarbage.db")
	s, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, p
}

func sampleTree() *tree.Tree {
	return &tree.Tree{
		Root: 0,
		Nodes: []tree.Node{
			{Name: `C:\`, Parent: tree.NoNode, Children: []tree.NodeID{1, 2}, OwnSize: 10, TotalSize: 1000, OwnFiles: 1, TotalFiles: 6, OwnLatest: 5, TreeLatest: 900},
			{Name: "Users", Parent: 0, Children: []tree.NodeID{3}, OwnSize: 0, TotalSize: 700, OwnFiles: 0, TotalFiles: 3, TreeLatest: 900},
			{Name: "Windows", Parent: 0, OwnSize: 290, TotalSize: 290, OwnFiles: 2, TotalFiles: 2, OwnLatest: 800, TreeLatest: 800, Flags: tree.FlagDenied | tree.FlagLink},
			{Name: "me", Parent: 1, OwnSize: 700, TotalSize: 700, OwnFiles: 3, TotalFiles: 3, OwnLatest: 900, TreeLatest: 900, Flags: tree.FlagCloud},
		},
		TopFiles:    []tree.FileInfo{{Path: `C:\Users\me\big.bin`, Size: 500, ModTime: 900}, {Path: `C:\a.txt`, Size: 10, ModTime: 5}},
		Extensions:  []tree.ExtStat{{Ext: ".bin", Count: 1, Bytes: 500}, {Ext: "", Count: 2, Bytes: 20}},
		Denied:      []string{`C:\System Volume Information`, `C:\Windows\CSC`},
		DeniedCount: 7,
	}
}

func TestSnapshotRoundTrip(t *testing.T) {
	s, _ := openTemp(t)
	ctx := context.Background()
	tr := sampleTree()
	at := time.UnixMilli(1700000000123).UTC()
	if err := s.SaveSnapshot(ctx, Snapshot{Volume: `C:\`, ScannedAt: at, Engine: "mft", Duration: 1500 * time.Millisecond, Tree: tr}); err != nil {
		t.Fatal(err)
	}
	got, err := s.LoadSnapshot(ctx, `C:\`)
	if err != nil || got == nil {
		t.Fatalf("load: %v %v", got, err)
	}
	if got.Volume != `C:\` || got.Engine != "mft" || got.Duration != 1500*time.Millisecond || !got.ScannedAt.Equal(at) || got.ScannedAt.Location() != time.UTC {
		t.Fatalf("meta mismatch: %+v", got)
	}
	if !reflect.DeepEqual(got.Tree, tr) {
		t.Fatalf("tree mismatch:\n got %+v\nwant %+v", got.Tree, tr)
	}
	if p := got.Tree.Path(3); p != `C:\Users\me` {
		t.Fatalf("path = %q", p)
	}
}

func countRows(t *testing.T, s *Store, table string) int {
	t.Helper()
	var n int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestSnapshotReplaces(t *testing.T) {
	s, _ := openTemp(t)
	ctx := context.Background()
	big := sampleTree()
	small := &tree.Tree{Root: 0, Nodes: []tree.Node{{Name: `C:\`, Parent: tree.NoNode, TotalSize: 1}}}
	if err := s.SaveSnapshot(ctx, Snapshot{Volume: `C:\`, ScannedAt: time.Now(), Tree: big}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSnapshot(ctx, Snapshot{Volume: `C:\`, ScannedAt: time.Now(), Tree: small}); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, s, "snapshot_nodes"); n != 1 {
		t.Fatalf("nodes = %d", n)
	}
	if n := countRows(t, s, "snapshots"); n != 1 {
		t.Fatalf("snapshots = %d", n)
	}
	if n := countRows(t, s, "snapshot_top_files"); n != 0 {
		t.Fatalf("top files = %d", n)
	}
	got, _ := s.LoadSnapshot(ctx, `C:\`)
	if len(got.Tree.Nodes) != 1 {
		t.Fatalf("loaded %d nodes", len(got.Tree.Nodes))
	}
}

func TestLoadSnapshotMissing(t *testing.T) {
	s, _ := openTemp(t)
	got, err := s.LoadSnapshot(context.Background(), `D:\`)
	if got != nil || err != nil {
		t.Fatalf("got %v %v", got, err)
	}
}

func TestLatestScans(t *testing.T) {
	s, _ := openTemp(t)
	ctx := context.Background()
	a := time.UnixMilli(1000).UTC()
	b := time.UnixMilli(2000).UTC()
	s.SaveSnapshot(ctx, Snapshot{Volume: `C:\`, ScannedAt: a, Tree: sampleTree()})
	s.SaveSnapshot(ctx, Snapshot{Volume: `D:\`, ScannedAt: b, Tree: sampleTree()})
	m, err := s.LatestScans(ctx)
	if err != nil || len(m) != 2 || !m[`C:\`].Equal(a) || !m[`D:\`].Equal(b) {
		t.Fatalf("%v %v", m, err)
	}
}

func TestCleanupsAndTotal(t *testing.T) {
	s, _ := openTemp(t)
	ctx := context.Background()
	base := time.UnixMilli(1700000000000).UTC()
	recs := []CleanupRecord{
		{Path: `C:\a`, Size: 100, Mode: "recycle", Source: "rule", RuleID: "r1", Status: "ok", Freed: 100, At: base},
		{Path: `C:\b`, Size: 5, Mode: "recycle", Source: "ai", Status: "skipped", Reason: "protected", Freed: 0, At: base.Add(time.Second)},
		{Path: `C:\c`, Size: 50, Mode: "delete", Source: "rule", RuleID: "r2", Status: "ok", Freed: 50, At: base.Add(2 * time.Second)},
	}
	for _, r := range recs {
		if err := s.AddCleanup(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	tot, err := s.TotalFreed(ctx)
	if err != nil || tot != 150 {
		t.Fatalf("total %d %v", tot, err)
	}
	l, err := s.ListCleanups(ctx, 10)
	if err != nil || len(l) != 3 {
		t.Fatalf("%v %v", l, err)
	}
	if l[0].Path != `C:\c` || l[1].Path != `C:\b` || l[2].Path != `C:\a` {
		t.Fatalf("order: %+v", l)
	}
	if !reflect.DeepEqual(l[2], recs[0]) || l[1].Reason != "protected" {
		t.Fatalf("fields: %+v", l)
	}
	if l, _ := s.ListCleanups(ctx, 2); len(l) != 2 {
		t.Fatalf("limit: %d", len(l))
	}
}

func TestTotalFreedEmpty(t *testing.T) {
	s, _ := openTemp(t)
	if tot, err := s.TotalFreed(context.Background()); err != nil || tot != 0 {
		t.Fatalf("%d %v", tot, err)
	}
}

func TestAIReportCache(t *testing.T) {
	s, _ := openTemp(t)
	ctx := context.Background()
	if _, ok, err := s.GetAIReport(ctx, "k"); ok || err != nil {
		t.Fatalf("%v %v", ok, err)
	}
	if err := s.PutAIReport(ctx, "k", "anthropic", "m", []byte(`{"a":1}`)); err != nil {
		t.Fatal(err)
	}
	if err := s.PutAIReport(ctx, "k", "anthropic", "m", []byte(`{"a":2}`)); err != nil {
		t.Fatal(err)
	}
	b, ok, err := s.GetAIReport(ctx, "k")
	if err != nil || !ok || string(b) != `{"a":2}` {
		t.Fatalf("%s %v %v", b, ok, err)
	}
}

func TestSettings(t *testing.T) {
	s, _ := openTemp(t)
	ctx := context.Background()
	if _, ok, err := s.GetSetting(ctx, "x"); ok || err != nil {
		t.Fatalf("%v %v", ok, err)
	}
	s.SetSetting(ctx, "x", []byte("1"))
	s.SetSetting(ctx, "x", []byte("2"))
	b, ok, err := s.GetSetting(ctx, "x")
	if err != nil || !ok || string(b) != "2" {
		t.Fatalf("%s %v %v", b, ok, err)
	}
}

func TestReopenKeepsData(t *testing.T) {
	s, p := openTemp(t)
	ctx := context.Background()
	s.SetSetting(ctx, "x", []byte("1"))
	s.SaveSnapshot(ctx, Snapshot{Volume: `C:\`, ScannedAt: time.Now(), Tree: sampleTree()})
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s2, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	if b, ok, _ := s2.GetSetting(ctx, "x"); !ok || string(b) != "1" {
		t.Fatal("setting lost")
	}
	if got, _ := s2.LoadSnapshot(ctx, `C:\`); got == nil {
		t.Fatal("snapshot lost")
	}
	if n := countRows(t, s2, "schema_version"); n != 1 {
		t.Fatalf("schema_version rows = %d, want 1", n)
	}
}

func TestSave100kNodes(t *testing.T) {
	s, _ := openTemp(t)
	const n = 100000
	nodes := make([]tree.Node, n)
	nodes[0] = tree.Node{Name: `C:\`, Parent: tree.NoNode}
	for i := 1; i < n; i++ {
		p := tree.NodeID((i - 1) / 10)
		nodes[i] = tree.Node{Name: "dir" + string(rune('a'+i%26)), Parent: p, OwnSize: int64(i), TotalSize: int64(i), OwnFiles: 1, TotalFiles: 1}
		nodes[p].Children = append(nodes[p].Children, tree.NodeID(i))
	}
	start := time.Now()
	if err := s.SaveSnapshot(context.Background(), Snapshot{Volume: `C:\`, ScannedAt: time.Now(), Tree: &tree.Tree{Root: 0, Nodes: nodes}}); err != nil {
		t.Fatal(err)
	}
	d := time.Since(start)
	t.Logf("saved %d nodes in %v", n, d)
	// The time limit only applies without -race; the race detector slows SQLite ~10x.
	if !raceEnabled && d > 5*time.Second {
		t.Fatalf("too slow: %v", d)
	}
	got, err := s.LoadSnapshot(context.Background(), `C:\`)
	if err != nil || len(got.Tree.Nodes) != n {
		t.Fatalf("load: %v", err)
	}
}
