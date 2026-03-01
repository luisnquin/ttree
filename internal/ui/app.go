package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
	"github.com/luisnquin/ttree/internal/db"
	"github.com/luisnquin/ttree/internal/model"
)

var (
	titleStyle     = lipgloss.NewStyle().Bold(true)
	statusStyleMap = map[string]lipgloss.Style{
		"done":    lipgloss.NewStyle().Foreground(lipgloss.Color("42")),
		"todo":    lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		"blocked": lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
	}
	defaultStatusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	selectedStyle      = lipgloss.NewStyle().Background(lipgloss.Color("236"))
	contextPanelStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2).Width(40)

	nodeColorMap = map[string]lipgloss.Style{
		"1": lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
		"2": lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
		"3": lipgloss.NewStyle().Foreground(lipgloss.Color("11")),
		"4": lipgloss.NewStyle().Foreground(lipgloss.Color("12")),
		"5": lipgloss.NewStyle().Foreground(lipgloss.Color("13")),
		"6": lipgloss.NewStyle().Foreground(lipgloss.Color("14")),
		"7": lipgloss.NewStyle().Foreground(lipgloss.Color("15")),
		"8": lipgloss.NewStyle().Foreground(lipgloss.Color("208")),
		"9": lipgloss.NewStyle().Foreground(lipgloss.Color("213")),
	}
)

type AppState int

const (
	StateNormal AppState = iota
	StateEditTitle
	StateEditStatus
	StateEditContext
	StateSelectColor
	StateConfirmDelete
)

type UIModel struct {
	db              *db.DB
	nodes           []*model.Node
	roots           []*model.Node
	expandedList    []*model.Node     // flat list of currently visible nodes
	effectiveColors map[string]string // map from node ID to its resolved color
	cursor          int
	expanded        map[string]bool

	state       AppState
	titleInput  textinput.Model
	statusInput textinput.Model
	contextArea textarea.Model

	insertingChild bool
	insertingSib   bool
	err            error
}

func New(dbInst *db.DB) (*UIModel, error) {
	ti := textinput.New()
	ti.Placeholder = "Title..."
	ti.Focus()

	si := textinput.New()
	si.Placeholder = "Status (done, todo, blocked, etc)..."

	ta := textarea.New()
	ta.Placeholder = "Context note..."

	m := &UIModel{
		db:          dbInst,
		expanded:    make(map[string]bool),
		titleInput:  ti,
		statusInput: si,
		contextArea: ta,
	}

	if err := m.loadNodes(); err != nil {
		return nil, err
	}

	return m, nil
}

func (m *UIModel) loadNodes() error {
	ctx := context.Background()
	nodes, err := m.db.GetNodes(ctx)
	if err != nil {
		return err
	}
	m.nodes = nodes
	m.roots = db.BuildTree(m.nodes)
	if err := m.updateExpandedList(); err != nil {
		return err
	}
	// default root expansion
	for _, root := range m.roots {
		m.expanded[root.ID] = true
	}
	m.updateExpandedList()
	return nil
}

func (m *UIModel) updateExpandedList() error {
	var list []*model.Node
	for _, root := range m.roots {
		m.traverse(root, &list)
	}
	m.expandedList = list

	// Create map to calculate effective colors
	// If a node has a color, use it. Otherwise, inherit from parent.
	m.effectiveColors = make(map[string]string)

	// We iterate through roots, and traverse down to set effective colors
	for _, root := range m.roots {
		m.calculateEffectiveColors(root, "")
	}

	if m.cursor >= len(m.expandedList) {
		m.cursor = len(m.expandedList) - 1
		if m.cursor < 0 {
			m.cursor = 0
		}
	}
	return nil
}

func (m *UIModel) traverse(n *model.Node, list *[]*model.Node) {
	*list = append(*list, n)
	if m.expanded[n.ID] {
		for _, child := range n.Children {
			m.traverse(child, list)
		}
	}
}

func (m *UIModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *UIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.state {
		case StateNormal:
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down", "j":
				if m.cursor < len(m.expandedList)-1 {
					m.cursor++
				}
			case "right", "l":
				if m.cursor < len(m.expandedList) {
					m.expanded[m.expandedList[m.cursor].ID] = true
					m.updateExpandedList()
				}
			case "left", "h":
				if m.cursor < len(m.expandedList) {
					m.expanded[m.expandedList[m.cursor].ID] = false
					m.updateExpandedList()
				}
			case "alt+up":
				m.moveNodeUp()
			case "alt+down":
				m.moveNodeDown()
			case "alt+left":
				m.moveNodeLeft()
			case "alt+right":
				m.moveNodeRight()
			case "a": // Add child
				m.insertingChild = true
				m.insertingSib = false
				m.state = StateEditTitle
				m.titleInput.SetValue("")
				m.titleInput.Focus()
				return m, textinput.Blink
			case "A": // Add sibling
				m.insertingChild = false
				m.insertingSib = true
				m.state = StateEditTitle
				m.titleInput.SetValue("")
				m.titleInput.Focus()
				return m, textinput.Blink
			case "e": // Edit title
				if m.cursor < len(m.expandedList) {
					m.state = StateEditTitle
					m.insertingChild = false
					m.insertingSib = false
					m.titleInput.SetValue(m.expandedList[m.cursor].Title)
					m.titleInput.Focus()
					return m, textinput.Blink
				}
			case "c": // Unlock color
				if m.cursor < len(m.expandedList) {
					m.state = StateSelectColor
				}
			case "y": // Yank node
				m.yankNode()
			case " ": // Edit status
				if m.cursor < len(m.expandedList) {
					m.state = StateEditStatus
					m.statusInput.SetValue(m.expandedList[m.cursor].Status)
					m.statusInput.Focus()
					return m, textinput.Blink
				}
			case "enter": // Edit context
				if m.cursor < len(m.expandedList) {
					m.state = StateEditContext
					m.contextArea.SetValue(m.expandedList[m.cursor].Context)
					m.contextArea.Focus()
					return m, textarea.Blink
				}
			case "x": // Delete node
				if m.cursor < len(m.expandedList) {
					m.state = StateConfirmDelete
				}
			}

		case StateConfirmDelete:
			switch msg.String() {
			case "y", "enter":
				m.state = StateNormal
				id := m.expandedList[m.cursor].ID
				m.db.DeleteNode(context.Background(), id)
				m.loadNodes()
			case "n", "esc":
				m.state = StateNormal
			}

		case StateEditTitle:
			switch msg.String() {
			case "enter":
				m.state = StateNormal
				m.titleInput.Blur()
				title := m.titleInput.Value()
				if title != "" {
					if m.insertingChild || m.insertingSib {
						var parentID *string
						if len(m.expandedList) > 0 {
							if m.insertingChild {
								parentID = &m.expandedList[m.cursor].ID
								m.expanded[m.expandedList[m.cursor].ID] = true // auto expand
							} else if m.insertingSib {
								parentID = m.expandedList[m.cursor].ParentID
							}
						}

						pos := 0 // append logic
						id := uuid.New().String()
						newNode := &model.Node{
							ID:        id,
							ParentID:  parentID,
							Title:     title,
							Position:  pos,
							CreatedAt: time.Now(),
						}
						m.db.CreateNode(context.Background(), newNode)
					} else {
						// updating existing
						node := m.expandedList[m.cursor]
						node.Title = title
						m.db.UpdateNode(context.Background(), node)
					}
				}
				m.insertingChild = false
				m.insertingSib = false
				m.loadNodes()
			case "esc":
				m.state = StateNormal
				m.titleInput.Blur()
				m.insertingChild = false
				m.insertingSib = false
			default:
				m.titleInput, cmd = m.titleInput.Update(msg)
				return m, cmd
			}

		case StateEditStatus:
			switch msg.String() {
			case "enter":
				m.state = StateNormal
				m.statusInput.Blur()
				node := m.expandedList[m.cursor]
				node.Status = m.statusInput.Value()
				m.db.UpdateNode(context.Background(), node)
				m.loadNodes()
			case "esc":
				m.state = StateNormal
				m.statusInput.Blur()
			default:
				m.statusInput, cmd = m.statusInput.Update(msg)
				return m, cmd
			}

		case StateEditContext:
			switch msg.String() {
			case "esc":
				m.state = StateNormal
				m.contextArea.Blur()
				node := m.expandedList[m.cursor]
				node.Context = m.contextArea.Value()
				m.db.UpdateNode(context.Background(), node)
				m.loadNodes()
			default:
				m.contextArea, cmd = m.contextArea.Update(msg)
				return m, cmd
			}

		case StateSelectColor:
			switch msg.String() {
			case "esc":
				m.state = StateNormal
			case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
				m.state = StateNormal
				node := m.expandedList[m.cursor]
				if msg.String() == "0" {
					node.Color = ""
				} else {
					node.Color = msg.String()
				}
				m.db.UpdateNode(context.Background(), node)
				m.loadNodes()
			}
		}
	}

	return m, nil
}

func (m *UIModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n", m.err)
	}

	var b strings.Builder
	b.WriteString("ttree (q/ctrl+c to quit, ←/→ to collapse/expand, ↑/↓ to navigate)\n")
	b.WriteString("a: Child | A: Sibling | e: Title | space: Status | enter: Context | x: Delete | c: Color\n\n")

	// Pre-calculate line prefixes (standard tree drawing logic)
	// Build map from ID -> prefix string
	prefixes := make(map[string]string)
	for i, root := range m.roots {
		isLast := i == len(m.roots)-1
		m.buildPrefixes(root, "", isLast, prefixes)
	}

	for i, node := range m.expandedList {
		prefix := prefixes[node.ID]
		if prefix == "" && node.ParentID != nil {
			// fallback if something went wrong, though shouldn't happen
			prefix = "  "
		}

		line := prefix

		// Node title & status
		title := node.Title

		var renderedTitle string
		effectiveColor := m.effectiveColors[node.ID]
		if effectiveColor != "" {
			if style, ok := nodeColorMap[effectiveColor]; ok {
				renderedTitle = style.Bold(true).Render(title)
			} else {
				renderedTitle = titleStyle.Render(title)
			}
		} else {
			renderedTitle = titleStyle.Render(title)
		}

		status := node.Status
		if status != "" {
			var stLip lipgloss.Style
			if val, ok := statusStyleMap[strings.ToLower(status)]; ok {
				stLip = val
			} else {
				stLip = defaultStatusStyle
			}
			title = fmt.Sprintf("%s | %s", renderedTitle, stLip.Render(status))
		} else {
			title = renderedTitle
		}

		if node.Context != "" {
			title += lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(" [...]")
		}

		if len(node.Children) > 0 {
			if m.expanded[node.ID] {
				line += "[-] "
			} else {
				line += "[+] "
			}
		} else {
			line += "    "
		}

		line += title

		if i == m.cursor {
			switch m.state {
			case StateEditTitle:
				line += "\n" + m.titleInput.View()
			case StateEditStatus:
				line += "\n" + m.statusInput.View()
			default:
				line = selectedStyle.Render(line)
			}
		}

		b.WriteString(line + "\n")
	}

	if m.state == StateNormal && m.cursor < len(m.expandedList) && m.expandedList[m.cursor].Context != "" {
		b.WriteString("\n" + contextPanelStyle.Render(m.expandedList[m.cursor].Context) + "\n")
	} else if m.state == StateEditContext {
		b.WriteString("\n" + contextPanelStyle.Render(m.contextArea.View()) + "\n")
		b.WriteString("(Press ESC to save and close context)\n")
	} else if m.state == StateConfirmDelete {
		b.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true).Render("Delete this node and all its children? (y/n)") + "\n")
	} else if len(m.expandedList) == 0 {
		b.WriteString("No tasks yet. Press 'a' to add one.\n")
		if m.state == StateEditTitle {
			b.WriteString("\n" + m.titleInput.View() + "\n")
		}
	}

	return b.String()
}

func (m *UIModel) buildPrefixes(node *model.Node, prefix string, isLast bool, acc map[string]string) {
	acc[node.ID] = prefix

	newPrefix := prefix
	if node.ParentID != nil {
		if isLast {
			newPrefix += "    "
		} else {
			newPrefix += "│   "
		}
	}

	for i, child := range node.Children {
		isChildLast := i == len(node.Children)-1
		childPrefix := newPrefix
		if isChildLast {
			childPrefix += "└── "
		} else {
			childPrefix += "├── "
		}
		m.buildPrefixes(child, childPrefix, isChildLast, acc)
	}
}

func (m *UIModel) calculateEffectiveColors(node *model.Node, inheritedColor string) {
	colorToPass := inheritedColor
	if node.Color != "" {
		colorToPass = node.Color
	}
	m.effectiveColors[node.ID] = colorToPass

	for _, child := range node.Children {
		m.calculateEffectiveColors(child, colorToPass)
	}
}

func (m *UIModel) moveNodeUp() {
	if m.cursor <= 0 || len(m.expandedList) == 0 {
		return
	}
	node := m.expandedList[m.cursor]
	siblings := m.getSiblings(node)
	if len(siblings) <= 1 {
		return
	}

	idx := -1
	for i, s := range siblings {
		if s.ID == node.ID {
			idx = i
			break
		}
	}

	if idx > 0 {
		prev := siblings[idx-1]
		node.Position, prev.Position = prev.Position, node.Position
		if node.Position == prev.Position {
			// fallback if they have same position
			for i, s := range siblings {
				s.Position = i
			}
			siblings[idx].Position, siblings[idx-1].Position = idx-1, idx
		}
		m.db.UpdateNode(context.Background(), node)
		m.db.UpdateNode(context.Background(), prev)
		m.loadNodes()
		// Re-find cursor
		for i, n := range m.expandedList {
			if n.ID == node.ID {
				m.cursor = i
				break
			}
		}
	}
}

func (m *UIModel) moveNodeDown() {
	if m.cursor < 0 || m.cursor >= len(m.expandedList) {
		return
	}
	node := m.expandedList[m.cursor]
	siblings := m.getSiblings(node)
	if len(siblings) <= 1 {
		return
	}

	idx := -1
	for i, s := range siblings {
		if s.ID == node.ID {
			idx = i
			break
		}
	}

	if idx != -1 && idx < len(siblings)-1 {
		next := siblings[idx+1]
		node.Position, next.Position = next.Position, node.Position
		if node.Position == next.Position {
			for i, s := range siblings {
				s.Position = i
			}
			siblings[idx].Position, siblings[idx+1].Position = idx+1, idx
		}
		m.db.UpdateNode(context.Background(), node)
		m.db.UpdateNode(context.Background(), next)
		m.loadNodes()
		for i, n := range m.expandedList {
			if n.ID == node.ID {
				m.cursor = i
				break
			}
		}
	}
}

func (m *UIModel) moveNodeLeft() {
	if m.cursor < 0 || m.cursor >= len(m.expandedList) {
		return
	}
	node := m.expandedList[m.cursor]
	if node.ParentID == nil {
		return
	}

	var parent *model.Node
	for _, n := range m.nodes {
		if n.ID == *node.ParentID {
			parent = n
			break
		}
	}

	if parent != nil {
		node.ParentID = parent.ParentID
		node.Position = parent.Position + 1
		// Shift others
		ctx := context.Background()
		for _, n := range m.nodes {
			if n.ParentID == node.ParentID && n.Position >= node.Position && n.ID != node.ID {
				n.Position++
				m.db.UpdateNode(ctx, n)
			}
		}
		m.db.UpdateNode(ctx, node)
		m.loadNodes()
		for i, n := range m.expandedList {
			if n.ID == node.ID {
				m.cursor = i
				break
			}
		}
	}
}

func (m *UIModel) moveNodeRight() {
	if m.cursor < 0 || m.cursor >= len(m.expandedList) {
		return
	}
	node := m.expandedList[m.cursor]
	siblings := m.getSiblings(node)

	idx := -1
	for i, s := range siblings {
		if s.ID == node.ID {
			idx = i
			break
		}
	}

	if idx > 0 {
		newParent := siblings[idx-1]
		node.ParentID = &newParent.ID
		node.Position = len(newParent.Children)
		m.expanded[newParent.ID] = true
		m.db.UpdateNode(context.Background(), node)
		m.loadNodes()
		for i, n := range m.expandedList {
			if n.ID == node.ID {
				m.cursor = i
				break
			}
		}
	} else if idx == 0 && len(siblings) > 1 {
		// Indent into the sibling below
		newParent := siblings[1]
		node.ParentID = &newParent.ID
		node.Position = 0 // first child
		m.expanded[newParent.ID] = true
		m.db.UpdateNode(context.Background(), node)
		m.loadNodes()
		for i, n := range m.expandedList {
			if n.ID == node.ID {
				m.cursor = i
				break
			}
		}
	}
}

func (m *UIModel) yankNode() {
	if m.cursor < 0 || m.cursor >= len(m.expandedList) {
		return
	}

	node := m.expandedList[m.cursor]
	var b strings.Builder
	m.formatNodeRecursive(node, 0, &b)

	clipboard.WriteAll(b.String())
}

func (m *UIModel) formatNodeRecursive(n *model.Node, level int, b *strings.Builder) {
	indent := strings.Repeat("  ", level)
	b.WriteString(fmt.Sprintf("%s- %s", indent, n.Title))
	if n.Status != "" {
		b.WriteString(fmt.Sprintf(" [%s]", n.Status))
	}
	b.WriteString("\n")

	for _, child := range n.Children {
		m.formatNodeRecursive(child, level+1, b)
	}
}

func (m *UIModel) getSiblings(node *model.Node) []*model.Node {
	var siblings []*model.Node
	for _, n := range m.nodes {
		if (node.ParentID == nil && n.ParentID == nil) || (node.ParentID != nil && n.ParentID != nil && *node.ParentID == *n.ParentID) {
			siblings = append(siblings, n)
		}
	}
	return siblings
}
