package model

import "time"

// Node represents an entry in the tree
type Node struct {
	ID        string    `json:"id"`
	ParentID  *string   `json:"parent_id"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	Context   string    `json:"context"`
	Position  int       `json:"position"`
	Color     string    `json:"color"`
	CreatedAt time.Time `json:"created_at"`
	Children  []*Node   `json:"children,omitempty"`
}
