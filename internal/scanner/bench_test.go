package scanner

import (
	"context"
	"fmt"
	"testing"

	"github.com/engviniciusbs/nodegarbage/internal/tree"
)

// BenchmarkWalkSynthetic: ~20k dirs, ~500k files in memory.
func BenchmarkWalkSynthetic(b *testing.B) {
	f := &fakeFS{dirs: map[string][]DirEntry{}, errs: map[string]error{}, forbid: map[string]bool{}}
	var top []DirEntry
	files := func() []DirEntry {
		es := make([]DirEntry, 25)
		for i := range es {
			es[i] = DirEntry{Name: fmt.Sprintf("file%d.dat", i), Size: int64(i * 100), ModTime: 1}
		}
		return es
	}
	for i := 0; i < 100; i++ {
		p := fmt.Sprintf(`C:\d%d`, i)
		top = append(top, DirEntry{Name: fmt.Sprintf("d%d", i), IsDir: true})
		var sub []DirEntry
		for j := 0; j < 200; j++ {
			sub = append(sub, DirEntry{Name: fmt.Sprintf("s%d", j), IsDir: true})
			f.dirs[fmt.Sprintf(`%s\s%d`, p, j)] = files()
		}
		f.dirs[p] = sub
	}
	f.dirs[`C:\`] = top
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		bl := tree.NewBuilder(`C:\`, 1000)
		if err := (&WalkScanner{FS: f}).Scan(context.Background(), `C:\`, bl, &Progress{}); err != nil {
			b.Fatal(err)
		}
		bl.Finish()
	}
}
