package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

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
)

type AppState int

const (
	StateNormal AppState = iota
	StateEditTitle
	StateEditStatus
	StateEditContext
)

type UIModel struct {
	db           *db.DB
	nodes        []*model.Node
	roots        []*model.Node
	expandedList []*model.Node // flat list of currently visible nodes
	cursor       int
	expanded     map[string]bool

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
					id := m.expandedList[m.cursor].ID
					m.db.DeleteNode(context.Background(), id)
					m.loadNodes()
				}
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
	b.WriteString("a: Child | A: Sibling | e: Title | space: Status | enter: Context | x: Delete\n\n")

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
		status := node.Status
		if status != "" {
			var stLip lipgloss.Style
			if val, ok := statusStyleMap[strings.ToLower(status)]; ok {
				stLip = val
			} else {
				stLip = defaultStatusStyle
			}
			title = fmt.Sprintf("%s | %s", titleStyle.Render(title), stLip.Render(status))
		} else {
			title = titleStyle.Render(title)
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
