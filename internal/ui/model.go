package ui

import (
	"context"
	"math"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

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
	paused      bool

	heapHistory []historyPoint
	stwP50Hist  []historyPoint
	stwP99Hist  []historyPoint

	cursor int

	pausedWindow   []domain.GCEvent
	pausedAgg      domain.Aggregates
	pausedHeapHist []historyPoint
	pausedSTWP50   []historyPoint
	pausedSTWP99   []historyPoint

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
		case "?", "h", "f1":
			m.helpVisible = !m.helpVisible
			return m, nil
		case "s":
			return m, takeSnapshotCmd(m.store.Recent(), m.agg, m.snapshotWriter)
		case " ":
			m.togglePause()
			return m, nil
		case "left":
			m.moveCursor(-1)
			return m, nil
		case "right":
			m.moveCursor(1)
			return m, nil
		case "home":
			m.setCursor(0)
			return m, nil
		case "end":
			m.setCursor(m.currentWindowLen() - 1)
			return m, nil
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
		m.pushHistory(msg.At)
		if !m.paused {
			m.cursor = m.currentWindowLen() - 1
		}
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

	const (
		paddingX = 2
		paddingY = 1
		gapX     = 2
		gapY     = 1
	)

	w := m.width
	h := m.height
	if w <= 0 {
		w = 120
	}
	if h <= 0 {
		h = 40
	}

	screen := Rect{W: w, H: h}
	content := Rect{
		X: paddingX,
		Y: paddingY,
		W: w - paddingX*2,
		H: h - paddingY*2,
	}
	if content.W < 0 {
		content.W = 0
	}
	if content.H < 0 {
		content.H = 0
	}

	// Fallback for narrow terminals: stack panels vertically.
	if content.W < 90 {
		rows := stackPanels(content, gapY, 4, []int{7, 7, 10, 8, 10, 12})
		if len(rows) == 0 {
			return lipgloss.NewStyle().Padding(paddingY, paddingX).Render("(terminal too small)")
		}

		window, agg, heapHist, p50Hist, p99Hist := m.displayData()

		current := renderCurrentValues(agg, rows[0].W, rows[0].H)
		parts := []string{current}

		if len(rows) > 1 {
			info := renderInformation(agg, m.now, m.lastUpdate, m.snapshotDir, m.lastSnapshot, rows[1].W, rows[1].H)
			parts = append(parts, info)
		}

		var visWindow []domain.GCEvent
		visCursor := 0
		if len(rows) > 2 {
			visWindow = m.visibleWindowForBar(window, rows[2].W, rows[2].H)
			visCursor = m.visibleCursorForBar(window, rows[2].W, rows[2].H)
			bar := renderSTWBarChart(visWindow, visCursor, 0, rows[2].H, rows[2].W)
			parts = append(parts, bar)
		}
		if len(rows) > 3 {
			details := renderCycleDetails(visWindow, visCursor, rows[3].W, rows[3].H)
			parts = append(parts, details)
		}
		if len(rows) > 4 {
			heap := renderHeapLiveHistory(heapHist, rows[4].W, rows[4].H)
			parts = append(parts, heap)
		}
		if len(rows) > 5 {
			stw := renderSTWPercentilesHistory(p50Hist, p99Hist, rows[5].W, rows[5].H)
			parts = append(parts, stw)
		}

		app := strings.Join(parts, strings.Repeat("\n", gapY))
		app = m.withFooter(app, content.W)
		app = fitViewport(app, content.W, content.H)
		_ = screen
		return lipgloss.NewStyle().Padding(paddingY, paddingX).Render(app)
	}

	// Height-based layout: scale rows to fit available height.
	// Priorities: row1 (current+info) > row2 (stw+heap) > row3 (stw p50/p99).
	rows := stackPanels(content, gapY, 6, []int{8, 12, 10})
	if len(rows) == 0 {
		return lipgloss.NewStyle().Padding(paddingY, paddingX).Render("(terminal too small)")
	}

	row1Cols := Cols(rows[0], 0.50, 0.50)

	// Apply gaps.
	row1Cols[0].W -= gapX / 2
	row1Cols[1].X += gapX / 2
	row1Cols[1].W -= gapX / 2
	if row1Cols[0].W < 0 {
		row1Cols[0].W = 0
	}
	if row1Cols[1].W < 0 {
		row1Cols[1].W = 0
	}

	window, agg, heapHist, p50Hist, p99Hist := m.displayData()

	current := renderCurrentValues(agg, row1Cols[0].W, row1Cols[0].H)
	info := renderInformation(agg, m.now, m.lastUpdate, m.snapshotDir, m.lastSnapshot, row1Cols[1].W, row1Cols[1].H)

	parts := []string{
		lipgloss.JoinHorizontal(lipgloss.Top, current, strings.Repeat(" ", gapX), info),
	}

	if len(rows) >= 2 {
		row2Cols := Cols(rows[1], 0.28, 0.22, 0.50)
		row2Cols[0].W -= gapX / 2
		row2Cols[1].X += gapX / 2
		row2Cols[1].W -= gapX / 2
		row2Cols[2].X += gapX / 2
		row2Cols[2].W -= gapX / 2
		for i := range row2Cols {
			if row2Cols[i].W < 0 {
				row2Cols[i].W = 0
			}
		}

		visWindow := m.visibleWindowForBar(window, row2Cols[0].W, row2Cols[0].H)
		visCursor := m.visibleCursorForBar(window, row2Cols[0].W, row2Cols[0].H)
		bar := renderSTWBarChart(visWindow, visCursor, 0, row2Cols[0].H, row2Cols[0].W)
		details := renderCycleDetails(visWindow, visCursor, row2Cols[1].W, row2Cols[1].H)
		heap := renderHeapLiveHistory(heapHist, row2Cols[2].W, row2Cols[2].H)

		parts = append(parts,
			lipgloss.JoinHorizontal(lipgloss.Top, bar, strings.Repeat(" ", gapX), details, strings.Repeat(" ", gapX), heap),
		)
	}

	if len(rows) >= 3 {
		stw := renderSTWPercentilesHistory(p50Hist, p99Hist, rows[2].W, rows[2].H)
		parts = append(parts, stw)
	}

	app := strings.Join(parts, strings.Repeat("\n", gapY))
	app = m.withFooter(app, content.W)
	app = fitViewport(app, content.W, content.H)

	_ = screen
	return lipgloss.NewStyle().Padding(paddingY, paddingX).Render(app)
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

func (m *Model) pushHistory(at time.Time) {
	if !m.agg.HasData {
		return
	}

	const limit = 180

	m.heapHistory = appendLimited(m.heapHistory, historyPoint{At: at, Value: float64(m.agg.Current.HeapLiveMB)}, limit)
	m.stwP50Hist = appendLimited(m.stwP50Hist, historyPoint{At: at, Value: float64(m.agg.Window.STWP50Us)}, limit)
	m.stwP99Hist = appendLimited(m.stwP99Hist, historyPoint{At: at, Value: float64(m.agg.Window.STWP99Us)}, limit)
}

func (m *Model) togglePause() {
	if m.paused {
		m.paused = false
		m.cursor = m.currentWindowLen() - 1
		m.pausedWindow = nil
		m.pausedAgg = domain.Aggregates{}
		m.pausedHeapHist = nil
		m.pausedSTWP50 = nil
		m.pausedSTWP99 = nil
		return
	}

	m.paused = true
	m.pausedWindow = m.store.Recent()
	m.pausedAgg = domain.ComputeAggregates(m.pausedWindow)
	m.pausedHeapHist = append([]historyPoint(nil), m.heapHistory...)
	m.pausedSTWP50 = append([]historyPoint(nil), m.stwP50Hist...)
	m.pausedSTWP99 = append([]historyPoint(nil), m.stwP99Hist...)
	m.cursor = len(m.pausedWindow) - 1
}

func (m *Model) currentWindowLen() int {
	if m.paused {
		return len(m.pausedWindow)
	}
	return m.store.Len()
}

func (m *Model) moveCursor(delta int) {
	if !m.paused {
		return
	}
	m.setCursor(m.cursor + delta)
}

func (m *Model) setCursor(v int) {
	if !m.paused {
		return
	}
	max := len(m.pausedWindow) - 1
	if max < 0 {
		m.cursor = 0
		return
	}
	if v < 0 {
		v = 0
	}
	if v > max {
		v = max
	}
	m.cursor = v
}

func (m *Model) displayData() ([]domain.GCEvent, domain.Aggregates, []historyPoint, []historyPoint, []historyPoint) {
	if m.paused {
		return m.pausedWindow, m.pausedAgg, m.pausedHeapHist, m.pausedSTWP50, m.pausedSTWP99
	}
	return m.store.Recent(), m.agg, m.heapHistory, m.stwP50Hist, m.stwP99Hist
}

func (m *Model) visibleWindowForBar(window []domain.GCEvent, w, h int) []domain.GCEvent {
	inner := InnerRect(boxStyle, Rect{W: w, H: h})
	maxBars := inner.W
	if maxBars < 10 {
		maxBars = 10
	}
	return lastN(window, maxBars)
}

func (m *Model) visibleCursorForBar(window []domain.GCEvent, w, h int) int {
	inner := InnerRect(boxStyle, Rect{W: w, H: h})
	maxBars := inner.W
	if maxBars < 10 {
		maxBars = 10
	}
	if len(window) == 0 {
		return 0
	}

	// In LIVE, cursor tracks last element; in PAUSED it is in absolute window coords.
	cursorAbs := len(window) - 1
	if m.paused {
		cursorAbs = m.cursor
		if cursorAbs < 0 {
			cursorAbs = 0
		}
		if cursorAbs >= len(window) {
			cursorAbs = len(window) - 1
		}
	}

	// visible window is lastN(window, maxBars)
	if len(window) <= maxBars {
		return cursorAbs
	}
	start := len(window) - maxBars
	if cursorAbs < start {
		cursorAbs = start
	}
	return cursorAbs - start
}

func (m *Model) withFooter(app string, w int) string {
	state := "LIVE"
	if m.paused {
		state = "PAUSED"
	}
	footer := lipgloss.NewStyle().Foreground(lipgloss.Color("#5f5f5f")).Render(state + "  q quit | s snapshot | space pause | left/right scrub | ? help")
	if w > 0 {
		footer = ansi.Truncate(footer, w, "")
	}
	return app + "\n" + footer
}

func appendLimited(s []historyPoint, v historyPoint, limit int) []historyPoint {
	s = append(s, v)
	if limit <= 0 {
		return s
	}
	if len(s) <= limit {
		return s
	}
	return s[len(s)-limit:]
}

// stackPanels builds vertical rows that always fit into content.H (including gaps).
func stackPanels(content Rect, gapY int, minH int, desired []int) []Rect {
	if len(desired) == 0 || content.H <= 0 || content.W <= 0 {
		return nil
	}
	if minH <= 0 {
		minH = 1
	}

	count := len(desired)
	available := content.H - gapY*(count-1)
	for count > 1 && available < minH*count {
		count--
		available = content.H - gapY*(count-1)
	}
	if available <= 0 {
		return []Rect{{X: content.X, Y: content.Y, W: content.W, H: content.H}}
	}

	desired = desired[:count]
	wanted := 0
	for _, v := range desired {
		if v > 0 {
			wanted += v
		}
	}
	if wanted == 0 {
		wanted = 1
	}

	heights := make([]int, 0, count)
	scale := float64(available) / float64(wanted)
	for _, v := range desired {
		h := int(math.Round(float64(v) * scale))
		if h < minH {
			h = minH
		}
		heights = append(heights, h)
	}

	sum := 0
	for _, h := range heights {
		sum += h
	}
	diff := available - sum
	for diff != 0 {
		adjusted := false
		for i := len(heights) - 1; i >= 0 && diff != 0; i-- {
			if diff > 0 {
				heights[i]++
				diff--
				adjusted = true
				continue
			}
			if heights[i] > minH {
				heights[i]--
				diff++
				adjusted = true
			}
		}
		if !adjusted {
			break
		}
	}

	rows := make([]Rect, 0, len(heights))
	y := content.Y
	for _, h := range heights {
		rows = append(rows, Rect{X: content.X, Y: y, W: content.W, H: h})
		y += h + gapY
	}
	return rows
}

// fitViewport trims output to the available terminal size to avoid scroll/wrap.
func fitViewport(s string, w, h int) string {
	if w <= 0 && h <= 0 {
		return s
	}

	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	if h > 0 && len(lines) > h {
		lines = lines[:h]
	}
	if w > 0 {
		for i, ln := range lines {
			lines[i] = ansi.Truncate(ln, w, "")
		}
	}

	return strings.Join(lines, "\n")
}
