package ui

import (
	"context"

	"github.com/luisnquin/ttree/internal/db"
	"github.com/luisnquin/ttree/internal/model"
)

type Command interface {
	Execute() error
	Undo() error
}

type UndoManager struct {
	undoStack []Command
	redoStack []Command
}

func NewUndoManager() *UndoManager {
	return &UndoManager{}
}

func (m *UndoManager) Execute(cmd Command) error {
	if err := cmd.Execute(); err != nil {
		return err
	}
	m.undoStack = append(m.undoStack, cmd)
	m.redoStack = nil // Clear redo stack on new action
	return nil
}

func (m *UndoManager) Undo() error {
	if len(m.undoStack) == 0 {
		return nil
	}
	cmd := m.undoStack[len(m.undoStack)-1]
	m.undoStack = m.undoStack[:len(m.undoStack)-1]
	if err := cmd.Undo(); err != nil {
		return err
	}
	m.redoStack = append(m.redoStack, cmd)
	return nil
}

func (m *UndoManager) Redo() error {
	if len(m.redoStack) == 0 {
		return nil
	}
	cmd := m.redoStack[len(m.redoStack)-1]
	m.redoStack = m.redoStack[:len(m.redoStack)-1]
	if err := cmd.Execute(); err != nil {
		return err
	}
	m.undoStack = append(m.undoStack, cmd)
	return nil
}

// Handles node creation
type CreateNodeCommand struct {
	db   *db.DB
	node *model.Node
}

func (c *CreateNodeCommand) Execute() error {
	return c.db.CreateNode(context.Background(), c.node)
}

func (c *CreateNodeCommand) Undo() error {
	return c.db.DeleteNode(context.Background(), c.node.ID)
}

// Handles node updates (reversibly)
type UpdateNodeCommand struct {
	db  *db.DB
	old *model.Node
	new *model.Node
}

func (c *UpdateNodeCommand) Execute() error {
	return c.db.UpdateNode(context.Background(), c.new)
}

func (c *UpdateNodeCommand) Undo() error {
	return c.db.UpdateNode(context.Background(), c.old)
}

// Handles node deletion (reversibly)
type DeleteNodeCommand struct {
	db    *db.DB
	nodes []*model.Node // self + subtree
}

func (c *DeleteNodeCommand) Execute() error {
	// Simple delete by ID (CASCADE handles children in DB, but we keep them for Undo)
	// Actually, if we use CASCADE, we need to be careful with Undo.
	// We'll store the entire subtree and re-insert them.
	return c.db.DeleteNode(context.Background(), c.nodes[0].ID)
}

func (c *DeleteNodeCommand) Undo() error {
	// Re-inserting nodes in order.
	// Since we want to preserve relationships, we should insert them such that parents are inserted before children.
	// But since we have ParentID pointers, we can just insert them all.
	ctx := context.Background()
	for _, n := range c.nodes {
		if err := c.db.CreateNode(ctx, n); err != nil {
			return err
		}
	}
	return nil
}

// Groups multiple commands
type CompositeCommand struct {
	cmds []Command
}

func (c *CompositeCommand) Execute() error {
	for _, cmd := range c.cmds {
		if err := cmd.Execute(); err != nil {
			return err
		}
	}
	return nil
}

func (c *CompositeCommand) Undo() error {
	for i := len(c.cmds) - 1; i >= 0; i-- {
		if err := c.cmds[i].Undo(); err != nil {
			return err
		}
	}
	return nil
}
