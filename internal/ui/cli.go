package ui

import (
	"context"
	"fmt"

	"github.com/luisnquin/ttree/internal/db"
	"github.com/luisnquin/ttree/internal/model"
)

func PrintTree(dbInst *db.DB) error {
	ctx := context.Background()
	nodes, err := dbInst.GetNodes(ctx)
	if err != nil {
		return err
	}

	roots := db.BuildTree(nodes)
	if len(roots) == 0 {
		fmt.Println("No tasks yet.")
		return nil
	}

	fmt.Println("ttree")

	for i, root := range roots {
		isLast := i == len(roots)-1
		printNode(root, "", isLast)
	}

	return nil
}

func printNode(n *model.Node, prefix string, isLast bool) {
	connector := "├── "
	if isLast {
		connector = "└── "
	}

	title := n.Title
	if n.Status != "" {
		title = fmt.Sprintf("%s (%s)", title, n.Status)
	}

	fmt.Printf("%s%s%s\n", prefix, connector, title)

	newPrefix := prefix
	if isLast {
		newPrefix += "    "
	} else {
		newPrefix += "│   "
	}

	for i, child := range n.Children {
		isChildLast := i == len(n.Children)-1
		printNode(child, newPrefix, isChildLast)
	}
}
