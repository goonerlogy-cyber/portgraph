package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/arshnah/portgraph/internal/graph"
	"github.com/arshnah/portgraph/internal/netpoll"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const pollInterval = 500 * time.Millisecond
const stepsPerTick = 4

var colorTCP = lipgloss.Color("#5fb3ff")
var colorUDP = lipgloss.Color("#ffb347")
var colorEdge = lipgloss.Color("#3a3a3a")
var colorLabel = lipgloss.Color("#e0e0e0")

type tickMsg struct{}
type pollMsg struct {
	conns []netpoll.Conn
	err   error
}

type Model struct {
	graph  *graph.Graph
	width  int
	height int
	conns  []netpoll.Conn
	err    error
}

func New() Model {
	return Model{graph: graph.New()}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(pollCmd(), tickCmd())
}

func pollCmd() tea.Cmd {
	return func() tea.Msg {
		conns, err := netpoll.Poll()
		return pollMsg{conns: conns, err: err}
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(pollInterval, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

func (m Model) canvasSize() (int, int) {
	h := m.height - 3
	if h < 1 {
		h = 1
	}
	return m.width, h
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}

	case pollMsg:
		m.conns = msg.conns
		m.err = msg.err
		w, h := m.canvasSize()
		m.graph.Update(m.conns, float64(w*2), float64(h*4))
		return m, pollCmd()

	case tickMsg:
		m.graph.Step(stepsPerTick)
		return m, tickCmd()
	}

	return m, nil
}

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "portgraph: waiting for terminal size...\n"
	}

	w, h := m.canvasSize()
	canvas := NewCanvas(w, h)

	for _, e := range m.graph.Edges {
		from, okF := m.graph.Nodes[e.From]
		to, okT := m.graph.Nodes[e.To]
		if !okF || !okT {
			continue
		}
		canvas.Line(int(from.X), int(from.Y), int(to.X), int(to.Y), colorEdge)
	}

	for _, n := range m.graph.Nodes {
		col := colorTCP
		if n.Proto == netpoll.UDP {
			col = colorUDP
		}
		radius := 1
		cx, cy := int(n.X), int(n.Y)
		for dy := -radius; dy <= radius; dy++ {
			for dx := -radius; dx <= radius; dx++ {
				canvas.SetPixel(cx+dx, cy+dy, col, true)
			}
		}
	}

	var b strings.Builder
	for y := range h {
		for x := range w {
			r, col := canvas.Cell(x, y)
			if col == "" {
				b.WriteRune(r)
			} else {
				b.WriteString(lipgloss.NewStyle().Foreground(col).Render(string(r)))
			}
		}
		b.WriteString("\n")
	}

	b.WriteString(strings.Repeat("─", w))
	b.WriteString("\n")
	b.WriteString(m.footer())

	return b.String()
}

func (m Model) footer() string {
	top := m.graph.TopNodes(10)
	var parts []string
	for _, n := range top {
		if n.Kind != graph.KindProcess {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s(%d)", n.Label, n.Weight))
	}

	labelStyle := lipgloss.NewStyle().Foreground(colorLabel)
	tcpStyle := lipgloss.NewStyle().Foreground(colorTCP)
	udpStyle := lipgloss.NewStyle().Foreground(colorUDP)

	legend := tcpStyle.Render("● tcp") + "  " + udpStyle.Render("● udp") + "  q: quit"
	top10 := labelStyle.Render(strings.Join(parts, "  "))

	if m.err != nil {
		return fmt.Sprintf("error: %v\n%s", m.err, legend)
	}

	return top10 + "\n" + legend
}
