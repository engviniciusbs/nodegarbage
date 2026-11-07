package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/engviniciusbs/nodegarbage/internal/tree"
)

// Snapshot is the latest scan of one volume.
type Snapshot struct {
	Volume    string
	ScannedAt time.Time
	Engine    string
	Duration  time.Duration
	Tree      *tree.Tree
}

// SaveSnapshot replaces any previous snapshot of the volume, in one transaction.
func (s *Store) SaveSnapshot(ctx context.Context, sn Snapshot) (err error) {
	if sn.Tree == nil {
		return errors.New("store: nil tree")
	}
	t := sn.Tree
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()
	if _, err = tx.ExecContext(ctx, `DELETE FROM snapshots WHERE volume=?`, sn.Volume); err != nil {
		return err
	}
	var totalSize int64
	var totalFiles uint32
	if t.Root >= 0 && int(t.Root) < len(t.Nodes) {
		totalSize, totalFiles = t.Nodes[t.Root].TotalSize, t.Nodes[t.Root].TotalFiles
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO snapshots(volume,scanned_at,engine,duration_ms,root_id,total_size,total_files,denied_count)
		VALUES(?,?,?,?,?,?,?,?)`, sn.Volume, sn.ScannedAt.UnixMilli(), sn.Engine, sn.Duration.Milliseconds(), int32(t.Root), totalSize, totalFiles, t.DeniedCount)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO snapshot_nodes(snapshot_id,node_id,parent_id,name,own_size,total_size,own_files,total_files,own_latest,tree_latest,flags)
		VALUES(?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	for i := range t.Nodes {
		n := &t.Nodes[i]
		if _, err = stmt.ExecContext(ctx, id, i, int32(n.Parent), n.Name, n.OwnSize, n.TotalSize, n.OwnFiles, n.TotalFiles, n.OwnLatest, n.TreeLatest, int(n.Flags)); err != nil {
			stmt.Close()
			return err
		}
	}
	stmt.Close()

	for i, f := range t.TopFiles {
		if _, err = tx.ExecContext(ctx, `INSERT INTO snapshot_top_files(snapshot_id,rank,path,size,mod_time) VALUES(?,?,?,?,?)`, id, i, f.Path, f.Size, f.ModTime); err != nil {
			return err
		}
	}
	for i, e := range t.Extensions {
		if _, err = tx.ExecContext(ctx, `INSERT INTO snapshot_extensions(snapshot_id,rank,ext,count,bytes) VALUES(?,?,?,?,?)`, id, i, e.Ext, e.Count, e.Bytes); err != nil {
			return err
		}
	}
	for i, p := range t.Denied {
		if _, err = tx.ExecContext(ctx, `INSERT INTO snapshot_denied(snapshot_id,rank,path) VALUES(?,?,?)`, id, i, p); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// LoadSnapshot returns the volume's snapshot, or nil, nil when none exists.
func (s *Store) LoadSnapshot(ctx context.Context, volume string) (*Snapshot, error) {
	var (
		id, scanned, durMS, rootID int64
		deniedCount                int
		sn                         Snapshot
	)
	err := s.db.QueryRowContext(ctx, `SELECT id,scanned_at,engine,duration_ms,root_id,denied_count FROM snapshots WHERE volume=?`, volume).
		Scan(&id, &scanned, &sn.Engine, &durMS, &rootID, &deniedCount)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	sn.Volume = volume
	sn.ScannedAt = time.UnixMilli(scanned).UTC()
	sn.Duration = time.Duration(durMS) * time.Millisecond
	t := &tree.Tree{Root: tree.NodeID(rootID), DeniedCount: deniedCount}
	sn.Tree = t

	if err := s.loadNodes(ctx, id, t); err != nil {
		return nil, err
	}
	if err := s.scanRows(ctx, `SELECT path,size,mod_time FROM snapshot_top_files WHERE snapshot_id=? ORDER BY rank`, id, func(r *sql.Rows) error {
		var f tree.FileInfo
		if err := r.Scan(&f.Path, &f.Size, &f.ModTime); err != nil {
			return err
		}
		t.TopFiles = append(t.TopFiles, f)
		return nil
	}); err != nil {
		return nil, err
	}
	if err := s.scanRows(ctx, `SELECT ext,count,bytes FROM snapshot_extensions WHERE snapshot_id=? ORDER BY rank`, id, func(r *sql.Rows) error {
		var e tree.ExtStat
		if err := r.Scan(&e.Ext, &e.Count, &e.Bytes); err != nil {
			return err
		}
		t.Extensions = append(t.Extensions, e)
		return nil
	}); err != nil {
		return nil, err
	}
	if err := s.scanRows(ctx, `SELECT path FROM snapshot_denied WHERE snapshot_id=? ORDER BY rank`, id, func(r *sql.Rows) error {
		var p string
		if err := r.Scan(&p); err != nil {
			return err
		}
		t.Denied = append(t.Denied, p)
		return nil
	}); err != nil {
		return nil, err
	}
	return &sn, nil
}

// scanRows runs a query and calls fn per row; the connection is released before returning.
func (s *Store) scanRows(ctx context.Context, q string, id int64, fn func(*sql.Rows) error) error {
	rows, err := s.db.QueryContext(ctx, q, id)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := fn(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

// loadNodes rebuilds Nodes (node_id == slice index) and Children from parent links.
func (s *Store) loadNodes(ctx context.Context, id int64, t *tree.Tree) error {
	return s.scanRows(ctx, `SELECT node_id,parent_id,name,own_size,total_size,own_files,total_files,own_latest,tree_latest,flags
		FROM snapshot_nodes WHERE snapshot_id=? ORDER BY node_id`, id, func(r *sql.Rows) error {
		var nid, parent int64
		var flags int
		var n tree.Node
		if err := r.Scan(&nid, &parent, &n.Name, &n.OwnSize, &n.TotalSize, &n.OwnFiles, &n.TotalFiles, &n.OwnLatest, &n.TreeLatest, &flags); err != nil {
			return err
		}
		if nid != int64(len(t.Nodes)) {
			return errors.New("store: snapshot nodes are not contiguous")
		}
		n.Parent = tree.NodeID(parent)
		n.Flags = tree.Flags(flags)
		if n.Parent != tree.NoNode {
			if n.Parent < 0 || int64(n.Parent) >= nid {
				return errors.New("store: snapshot node has invalid parent")
			}
			p := &t.Nodes[n.Parent]
			p.Children = append(p.Children, tree.NodeID(nid))
		}
		t.Nodes = append(t.Nodes, n)
		return nil
	})
}

// LatestScans maps each volume to the time of its stored snapshot.
func (s *Store) LatestScans(ctx context.Context) (map[string]time.Time, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT volume, scanned_at FROM snapshots`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]time.Time{}
	for rows.Next() {
		var v string
		var ms int64
		if err := rows.Scan(&v, &ms); err != nil {
			return nil, err
		}
		out[v] = time.UnixMilli(ms).UTC()
	}
	return out, rows.Err()
}
