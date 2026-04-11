package diffviewer

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/WhiteAbeLincoln/git-different/pkg/icons"
	"github.com/WhiteAbeLincoln/git-different/pkg/ui/common"
	"github.com/WhiteAbeLincoln/git-different/pkg/utils"
)

const dirHeaderHeight = 3

type Model struct {
	preamble string
	vp       viewport.Model
	header   headerData
	common.Common
}

type headerData struct {
	path      string
	isDir     bool
	additions int64
	deletions int64
}

func New() Model {
	return Model{
		vp: viewport.Model{},
	}
}

func (m *Model) SetPreamble(preamble string) {
	m.preamble = preamble
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "down", "j", "n", "up", "k", "N", "p":
			// Consumed by main model for tree navigation
		default:
			vp, cmd := m.vp.Update(msg)
			m.vp = vp
			return m, cmd
		}
	}
	return m, nil
}

const scrollbarWidth = 3

func (m Model) View() string {
	vpView := m.vp.View()
	scrollbar := common.RenderScrollbar(m.vp.Height(), m.vp.TotalLineCount(), m.vp.YOffset())
	if scrollbar != "" {
		vpView = lipgloss.JoinHorizontal(lipgloss.Top, vpView, " ", scrollbar)
	}
	return lipgloss.JoinVertical(lipgloss.Left, m.headerView(), vpView)
}

// SetContent sets the rendered diff text directly into the viewport.
func (m *Model) SetContent(content string) {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if lipgloss.Width(line) > m.vp.Width() && m.vp.Width() > 0 {
			lines[i] = ansi.Truncate(line, m.vp.Width(), "")
		}
	}
	m.vp.SetContent(strings.Join(lines, "\n"))
}

// SetFileHeader configures the header for a single file diff.
func (m *Model) SetFileHeader(path string, additions, deletions int64) {
	m.header = headerData{path: path, isDir: false, additions: additions, deletions: deletions}
}

// SetDirHeader configures the header for a directory diff.
func (m *Model) SetDirHeader(path string, additions, deletions int64) {
	m.header = headerData{path: path, isDir: true, additions: additions, deletions: deletions}
}

func (m *Model) SetSize(width, height int) {
	m.Width = width
	m.Height = height
	m.vp.SetWidth(m.contentWidth())
	m.vp.SetHeight(m.Height - dirHeaderHeight)
}

func (m Model) contentWidth() int {
	return m.Width - scrollbarWidth
}

func (m *Model) GoToTop() {
	m.vp.GotoTop()
}

func (m *Model) ScrollUp(lines int) {
	m.vp.ScrollUp(lines)
}

func (m *Model) ScrollDown(lines int) {
	m.vp.ScrollDown(lines)
}

func (m Model) headerView() string {
	if m.header.isDir {
		return m.dirHeaderView()
	}
	if m.header.path == "" {
		return ""
	}

	base := lipgloss.NewStyle()
	fileIcon := icons.GetIcon(m.header.path, false)
	prefix := base.Render(fileIcon) + base.Render(" ")
	name := utils.TruncateString(m.header.path, m.Width-lipgloss.Width(prefix))
	top := prefix + base.Bold(true).Render(name)
	bottom := viewDiffStats(m.header.additions, m.header.deletions, base)

	return base.
		Width(m.Width).
		Height(dirHeaderHeight - 1).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(lipgloss.Color("8")).
		Render(lipgloss.JoinVertical(lipgloss.Left, top, bottom))
}

func (m Model) dirHeaderView() string {
	base := lipgloss.NewStyle().Foreground(lipgloss.Blue)
	prefix := base.Render(" ")
	name := utils.TruncateString(m.header.path, m.Width-lipgloss.Width(prefix))
	top := prefix + base.Bold(true).Render(name)
	bottom := viewDiffStats(m.header.additions, m.header.deletions, base)

	return base.
		Width(m.Width).
		Height(dirHeaderHeight - 1).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(lipgloss.Color("8")).
		Render(lipgloss.JoinVertical(lipgloss.Left, top, bottom))
}

func viewDiffStats(added, deleted int64, base lipgloss.Style) string {
	var parts []string
	if added > 0 {
		parts = append(parts, base.Foreground(lipgloss.Green).Render(fmt.Sprintf("+%d", added)))
	}
	if deleted > 0 {
		parts = append(parts, base.Foreground(lipgloss.Red).Render(fmt.Sprintf("-%d", deleted)))
	}
	return strings.Join(parts, base.Render(" "))
}

func renderPreamble(preamble string) string {
	preamble = strings.TrimSpace(preamble)
	if preamble == "" {
		return ""
	}

	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	yellow := lipgloss.NewStyle().Foreground(lipgloss.Yellow)

	var out []string
	for _, line := range strings.Split(preamble, "\n") {
		switch {
		case strings.HasPrefix(line, "commit "):
			out = append(
				out,
				dim.Render("commit ")+yellow.Render(strings.TrimPrefix(line, "commit ")),
			)
		case strings.HasPrefix(line, "Author:"),
			strings.HasPrefix(line, "AuthorDate:"),
			strings.HasPrefix(line, "Date:"),
			strings.HasPrefix(line, "Commit:"),
			strings.HasPrefix(line, "CommitDate:"),
			strings.HasPrefix(line, "Merge:"):
			out = append(out, dim.Render(line))
		default:
			out = append(out, line)
		}
	}

	return strings.Join(out, "\n")
}
