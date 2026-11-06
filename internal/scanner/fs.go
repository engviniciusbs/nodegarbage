package scanner

// DirEntry is one child of a directory. ModTime is unix seconds. Attrs are
// Win32 file attributes. For files Size is the logical size.
type DirEntry struct {
	Name    string
	IsDir   bool
	Size    int64
	ModTime int64
	Attrs   uint32
}

// FS abstracts directory listing so the walker can be tested without a disk.
// ReadDir receives plain paths (no \\?\ prefix).
type FS interface {
	ReadDir(path string) ([]DirEntry, error)
}
