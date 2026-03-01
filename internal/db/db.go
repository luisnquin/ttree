package db

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"

	"github.com/luisnquin/ttree/internal/model"
	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	*sql.DB
}

func getDataDir() (string, error) {
	if xdgDataHome := os.Getenv("XDG_DATA_HOME"); xdgDataHome != "" {
		return filepath.Join(xdgDataHome, "ttree"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "ttree"), nil
}

func Open() (*DB, error) {
	dataDir, err := getDataDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}
	dbPath := filepath.Join(dataDir, "ttree.db")

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return &DB{db}, nil
}

func (db *DB) InitSchema(ctx context.Context) error {
	query := `
	CREATE TABLE IF NOT EXISTS nodes (
		id TEXT PRIMARY KEY,
		parent_id TEXT NULL,
		title TEXT NOT NULL,
		status TEXT,
		context TEXT,
		position INTEGER NOT NULL,
		color TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(parent_id) REFERENCES nodes(id) ON DELETE CASCADE
	);
	`
	_, err := db.ExecContext(ctx, query)
	if err != nil {
		return err
	}

	// Simple migration if table exists without color column
	db.ExecContext(ctx, "ALTER TABLE nodes ADD COLUMN color TEXT DEFAULT ''")

	return nil
}

func (db *DB) GetNodes(ctx context.Context) ([]*model.Node, error) {
	query := `SELECT id, parent_id, title, status, context, position, color, created_at FROM nodes ORDER BY position ASC, created_at ASC`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []*model.Node
	for rows.Next() {
		var n model.Node
		if err := rows.Scan(&n.ID, &n.ParentID, &n.Title, &n.Status, &n.Context, &n.Position, &n.Color, &n.CreatedAt); err != nil {
			return nil, err
		}
		nodes = append(nodes, &n)
	}
	return nodes, nil
}

func (db *DB) CreateNode(ctx context.Context, n *model.Node) error {
	query := `INSERT INTO nodes (id, parent_id, title, status, context, position, color, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := db.ExecContext(ctx, query, n.ID, n.ParentID, n.Title, n.Status, n.Context, n.Position, n.Color, n.CreatedAt)
	return err
}

func (db *DB) UpdateNode(ctx context.Context, n *model.Node) error {
	query := `UPDATE nodes SET parent_id=?, title=?, status=?, context=?, position=?, color=? WHERE id=?`
	_, err := db.ExecContext(ctx, query, n.ParentID, n.Title, n.Status, n.Context, n.Position, n.Color, n.ID)
	return err
}

func (db *DB) DeleteNode(ctx context.Context, id string) error {
	query := `DELETE FROM nodes WHERE id=?`
	_, err := db.ExecContext(ctx, query, id)
	return err
}

// BuildTree taking a flat slice of nodes
func BuildTree(nodes []*model.Node) []*model.Node {
	nodeMap := make(map[string]*model.Node)
	for _, n := range nodes {
		nodeMap[n.ID] = n
	}

	var roots []*model.Node
	for _, n := range nodes {
		if n.ParentID == nil {
			roots = append(roots, n)
		} else {
			if parent, ok := nodeMap[*n.ParentID]; ok {
				parent.Children = append(parent.Children, n)
			} else {
				// Parent not found, treat as root to avoid losing data
				roots = append(roots, n)
			}
		}
	}
	return roots
}
