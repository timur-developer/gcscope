package ui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/timur-developer/gcviz/internal/domain"
)

type Model struct {
	ctx    context.Context
	cancel context.CancelFunc

	store *domain.Store

	lastUpdate time.Time
	now        time.Time
	agg        domain.Aggregates

	width  int
	height int

	helpVisible bool

	snapshotWriter SnapshotWriter
	snapshotDir    string
	lastSnapshot   snapshotStatus
}

type GCEventMsg struct {
	Event domain.GCEvent
	At    time.Time
}

type SnapshotWriter interface {
	WriteSnapshot(events []domain.GCEvent, agg domain.Aggregates) (fileName string, err error)
}

type snapshotStatus struct {
	FileName string
	ErrMsg   string
}

type snapshotResultMsg snapshotStatus

func NewModel(ctx context.Context, cancel context.CancelFunc, windowSize int, snapshotDir string, snapshotWriter SnapshotWriter) Model {
	return Model{
		ctx:            ctx,
		cancel:         cancel,
		store:          domain.NewStore(windowSize),
		now:            time.Now(),
		snapshotDir:    snapshotDir,
		snapshotWriter: snapshotWriter,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(waitContextDone(m.ctx), tick())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.cancel()
			return m, tea.Quit
		case "?":
			m.helpVisible = !m.helpVisible
			return m, nil
		case "s":
			return m, takeSnapshotCmd(m.store.Recent(), m.agg, m.snapshotWriter)
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case GCEventMsg:
		m.lastUpdate = msg.At
		m.now = msg.At
		m.store.Add(msg.Event)
		m.agg = domain.ComputeAggregates(m.store.Recent())
		return m, nil
	case tickMsg:
		m.now = msg.At
		return m, tick()
	case contextDoneMsg:
		return m, tea.Quit
	case snapshotResultMsg:
		m.lastSnapshot = snapshotStatus(msg)
		return m, nil
	}

	return m, nil
}

func (m Model) View() string {
	if m.helpVisible {
		return renderHelp(m.width, m.height)
	}

	current := renderCurrentValues(m.agg)
	info := renderInformation(m.agg, m.now, m.lastUpdate, m.snapshotDir, m.lastSnapshot)
	bar := renderSTWBarChart(m.store.Recent(), barChartWidth, barChartHeight)

	panels := joinPanels(m.width, current, info)

	app := lipgloss.JoinVertical(lipgloss.Left, bar, panels)
	return lipgloss.NewStyle().Padding(1, 2).Render(app)
}

func (m Model) SnapshotState() ([]domain.GCEvent, domain.Aggregates) {
	return m.store.Recent(), m.agg
}

type contextDoneMsg struct{}

func waitContextDone(ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		<-ctx.Done()
		return contextDoneMsg{}
	}
}

type tickMsg struct{ At time.Time }

func tick() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg{At: t}
	})
}

func takeSnapshotCmd(events []domain.GCEvent, agg domain.Aggregates, w SnapshotWriter) tea.Cmd {
	if w == nil {
		return nil
	}
	if len(events) == 0 {
		return nil
	}

	return func() tea.Msg {
		name, err := w.WriteSnapshot(events, agg)
		if err != nil {
			return snapshotResultMsg(snapshotStatus{ErrMsg: err.Error()})
		}
		return snapshotResultMsg(snapshotStatus{FileName: name})
	}
}
