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
	layout      layoutMode
	stwTh       STWThresholds
	targetEnv   *TargetEnvInfo

	heapHistory []historyPoint
	stwP50Hist  []historyPoint
	stwP99Hist  []historyPoint
	stwMaxHist  []historyPoint

	cursor        int
	stwLabelsMode stwLabelMode

	pausedWindow   []domain.GCEvent
	pausedAgg      domain.Aggregates
	pausedHeapHist []historyPoint
	pausedSTWP50   []historyPoint
	pausedSTWP99   []historyPoint
	pausedSTWMax   []historyPoint

	snapshotWriter SnapshotWriter
	snapshotDir    string
	lastSnapshot   snapshotStatus

	manualSnapshotInFlight bool
	lastManualSnapshotAt   time.Time

	chartFocus chartFocus
	xSpan      xSpanMode
	heapYZoom  int
	stwYZoom   int
	heapYPan   int
	stwYPan    int
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

type stwLabelMode int

const (
	stwLabelGCAndSTW stwLabelMode = iota
	stwLabelGCAndHeap
	stwLabelGCOnly
)

type layoutMode int

const (
	layoutSpaced layoutMode = iota
	layoutTight
)

func (l layoutMode) next() layoutMode {
	if l == layoutSpaced {
		return layoutTight
	}
	return layoutSpaced
}

func (m stwLabelMode) next() stwLabelMode {
	switch m {
	case stwLabelGCAndSTW:
		return stwLabelGCAndHeap
	case stwLabelGCAndHeap:
		return stwLabelGCOnly
	default:
		return stwLabelGCAndSTW
	}
}

type chartFocus int

const (
	chartHeap chartFocus = iota
	chartSTW
)

func (c chartFocus) next() chartFocus {
	if c == chartHeap {
		return chartSTW
	}
	return chartHeap
}

type xSpanMode int

const (
	xSpanAll xSpanMode = iota
	xSpan1h
	xSpan15m
	xSpan5m
	xSpan1m
)

func (m xSpanMode) zoomIn() xSpanMode {
	if m >= xSpan1m {
		return xSpan1m
	}
	if m == xSpanAll {
		return xSpan1h
	}
	return m + 1
}

func (m xSpanMode) zoomOut() xSpanMode {
	if m <= xSpanAll {
		return xSpanAll
	}
	return m - 1
}

func (m xSpanMode) duration() time.Duration {
	switch m {
	case xSpan1m:
		return time.Minute
	case xSpan5m:
		return 5 * time.Minute
	case xSpan15m:
		return 15 * time.Minute
	case xSpan1h:
		return time.Hour
	default:
		return 0
	}
}

type TargetEnvInfo struct {
	GOGC      string
	GOMEMLIMIT string
	GODEBUG   string
}

func NewModel(ctx context.Context, cancel context.CancelFunc, windowSize int, snapshotDir string, snapshotWriter SnapshotWriter, stwTh STWThresholds, targetEnv *TargetEnvInfo) Model {
	if stwTh.BadUs <= stwTh.WarnUs {
		stwTh = STWThresholds{WarnUs: 200, BadUs: 1000}
	}
	return Model{
		ctx:            ctx,
		cancel:         cancel,
		store:          domain.NewStore(windowSize),
		now:            time.Now(),
		snapshotDir:    snapshotDir,
		snapshotWriter: snapshotWriter,
		stwLabelsMode:  stwLabelGCAndSTW,
		layout:         layoutSpaced,
		stwTh:          stwTh,
		targetEnv:      targetEnv,
		chartFocus:     chartHeap,
		xSpan:          xSpanAll,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(waitContextDone(m.ctx), tick())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyRunes && !msg.Paste && !msg.Alt {
			var cmds []tea.Cmd
			for _, r := range msg.Runes {
				switch r {
				case 'q':
					m.cancel()
					return m, tea.Quit
				case ' ':
					m.togglePause()
					m.clampZoomState()
				case '?', 'h':
					m.helpVisible = !m.helpVisible
				case 's':
					m.manualSnapshotInFlight = true
					cmds = append(cmds, takeSnapshotCmd(m.store.Recent(), m.agg, m.snapshotWriter))
				case 'l':
					m.stwLabelsMode = m.stwLabelsMode.next()
				case 'g':
					m.layout = m.layout.next()
				case 'z':
					m.chartFocus = m.chartFocus.next()
				case '+', '=':
					const maxZoomSteps = 8
					switch m.chartFocus {
					case chartHeap:
						if m.heapYZoom < maxZoomSteps {
							m.heapYZoom++
						}
					case chartSTW:
						if m.stwYZoom < maxZoomSteps {
							m.stwYZoom++
						}
					}
					m.clampZoomState()
				case '-':
					switch m.chartFocus {
					case chartHeap:
						if m.heapYZoom > 0 {
							m.heapYZoom--
						}
					case chartSTW:
						if m.stwYZoom > 0 {
							m.stwYZoom--
						}
					}
					m.clampZoomState()
				case '0':
					switch m.chartFocus {
					case chartHeap:
						m.heapYZoom = 0
						m.heapYPan = 0
					case chartSTW:
						m.stwYZoom = 0
						m.stwYPan = 0
					}
					m.clampZoomState()
				case '[':
					m.xSpan = m.xSpan.zoomIn()
					m.clampZoomState()
				case ']':
					m.xSpan = m.xSpan.zoomOut()
					m.clampZoomState()
				case 'r':
					m.chartFocus = chartHeap
					m.xSpan = xSpanAll
					m.heapYZoom = 0
					m.stwYZoom = 0
					m.heapYPan = 0
					m.stwYPan = 0
				}
			}
			return m, tea.Batch(cmds...)
		}

		switch msg.String() {
		case "ctrl+c", "q":
			m.cancel()
			return m, tea.Quit
		case "?", "h", "f1":
			m.helpVisible = !m.helpVisible
			return m, nil
		case "s":
			m.manualSnapshotInFlight = true
			return m, takeSnapshotCmd(m.store.Recent(), m.agg, m.snapshotWriter)
		case "l":
			m.stwLabelsMode = m.stwLabelsMode.next()
			return m, nil
		case "g":
			m.layout = m.layout.next()
			return m, nil
		case "z":
			m.chartFocus = m.chartFocus.next()
			return m, nil
		case "+", "=":
			const maxZoomSteps = 8
			switch m.chartFocus {
			case chartHeap:
				if m.heapYZoom < maxZoomSteps {
					m.heapYZoom++
				}
			case chartSTW:
				if m.stwYZoom < maxZoomSteps {
					m.stwYZoom++
				}
			}
			m.clampZoomState()
			return m, nil
		case "-":
			switch m.chartFocus {
			case chartHeap:
				if m.heapYZoom > 0 {
					m.heapYZoom--
				}
			case chartSTW:
				if m.stwYZoom > 0 {
					m.stwYZoom--
				}
			}
			m.clampZoomState()
			return m, nil
		case "0":
			switch m.chartFocus {
			case chartHeap:
				m.heapYZoom = 0
				m.heapYPan = 0
			case chartSTW:
				m.stwYZoom = 0
				m.stwYPan = 0
			}
			m.clampZoomState()
			return m, nil
		case "shift+up", "ctrl+up", "ctrl+shift+up":
			switch m.chartFocus {
			case chartHeap:
				m.heapYPan++
			case chartSTW:
				m.stwYPan++
			}
			m.clampZoomState()
			return m, nil
		case "shift+down", "ctrl+down", "ctrl+shift+down":
			switch m.chartFocus {
			case chartHeap:
				m.heapYPan--
			case chartSTW:
				m.stwYPan--
			}
			m.clampZoomState()
			return m, nil
		case "r":
			m.chartFocus = chartHeap
			m.xSpan = xSpanAll
			m.heapYZoom = 0
			m.stwYZoom = 0
			m.heapYPan = 0
			m.stwYPan = 0
			return m, nil
		case "[":
			m.xSpan = m.xSpan.zoomIn()
			m.clampZoomState()
			return m, nil
		case "]":
			m.xSpan = m.xSpan.zoomOut()
			m.clampZoomState()
			return m, nil
		case " ":
			m.togglePause()
			m.clampZoomState()
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
		m.clampZoomState()
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
		if m.manualSnapshotInFlight {
			m.manualSnapshotInFlight = false
			if m.lastSnapshot.FileName != "" && m.lastSnapshot.ErrMsg == "" {
				m.lastManualSnapshotAt = m.now
			}
		}
		return m, nil
	}

	return m, nil
}

func (m Model) HasRecentManualSnapshot(now time.Time, within time.Duration) bool {
	if m.lastManualSnapshotAt.IsZero() {
		return false
	}
	if within <= 0 {
		return true
	}
	return now.Sub(m.lastManualSnapshotAt) <= within
}

func (m Model) View() string {
	if m.helpVisible {
		return renderHelp(m.width, m.height)
	}

	if m.layout == layoutTight {
		return m.viewTight()
	}
	return m.viewSpaced()
}

func (m Model) viewSpaced() string {
	const (
		paddingX = 0
		paddingY = 0
		gapX     = 1
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

	// Reserve one line for the footer so panel borders don't get truncated.
	footerH := 1
	contentPanels := content
	if contentPanels.H > footerH {
		contentPanels.H -= footerH
	} else {
		contentPanels.H = 0
	}

	// Fallback for narrow terminals: stack panels vertically.
	if content.W < 90 {
		rows := stackPanels(contentPanels, gapY, 4, []int{7, 7, 10, 8, 10, 12})
		if len(rows) == 0 {
			return lipgloss.NewStyle().Padding(paddingY, paddingX).Render("(terminal too small)")
		}

		window, agg, heapHist, p50Hist, p99Hist, maxHist := m.displayData()

		current := renderCurrentValues(agg, frameBoxed, m.stwTh, rows[0].W, rows[0].H)
		parts := []string{current}

		if len(rows) > 1 {
			info := renderInformation(window, agg, m.now, m.lastUpdate, m.snapshotDir, m.lastSnapshot, frameBoxed, m.stwTh, m.targetEnv, rows[1].W, rows[1].H)
			parts = append(parts, info)
		}

		var visWindow []domain.GCEvent
		visCursor := 0
		if len(rows) > 2 {
			visWindow, visCursor = m.barViewport(window, frameBoxed, rows[2].W, rows[2].H)
			bar := renderSTWBarChart(visWindow, visCursor, frameBoxed, m.stwLabelsMode, m.stwTh, 0, rows[2].H, rows[2].W)
			parts = append(parts, bar)
		}
		if len(rows) > 3 {
			details := renderCycleDetails(visWindow, visCursor, frameBoxed, m.stwTh, rows[3].W, rows[3].H)
			parts = append(parts, details)
		}
		if len(rows) > 4 {
			heap := renderHeapLiveHistory(heapHist, frameBoxed, chartView{XSpan: m.xSpan.duration(), YZoomSteps: m.heapYZoom, YPanSteps: m.heapYPan, Focused: m.chartFocus == chartHeap}, rows[4].W, rows[4].H)
			parts = append(parts, heap)
		}
		if len(rows) > 5 {
			stw := renderSTWPercentilesHistory(p50Hist, p99Hist, maxHist, frameBoxed, chartView{XSpan: m.xSpan.duration(), YZoomSteps: m.stwYZoom, YPanSteps: m.stwYPan, Focused: m.chartFocus == chartSTW}, rows[5].W, rows[5].H)
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
	rows := stackPanels(contentPanels, gapY, 6, []int{8, 12, 10})
	if len(rows) == 0 {
		return lipgloss.NewStyle().Padding(paddingY, paddingX).Render("(terminal too small)")
	}

	row1AvailW := rows[0].W - gapX
	if row1AvailW < 0 {
		row1AvailW = 0
	}
	row1Cols := Cols(Rect{W: row1AvailW, H: rows[0].H}, 0.50, 0.50)

	window, agg, heapHist, p50Hist, p99Hist, maxHist := m.displayData()

	current := renderCurrentValues(agg, frameBoxed, m.stwTh, row1Cols[0].W, row1Cols[0].H)
	info := renderInformation(window, agg, m.now, m.lastUpdate, m.snapshotDir, m.lastSnapshot, frameBoxed, m.stwTh, m.targetEnv, row1Cols[1].W, row1Cols[1].H)

	parts := []string{
		lipgloss.JoinHorizontal(lipgloss.Top, current, strings.Repeat(" ", gapX), info),
	}

	if len(rows) >= 2 {
		row2AvailW := rows[1].W - gapX*2
		if row2AvailW < 0 {
			row2AvailW = 0
		}
		// Give the bar chart and details more room; heap chart is still readable at ~40%.
		row2Cols := Cols(Rect{W: row2AvailW, H: rows[1].H}, 0.36, 0.24, 0.40)

		visWindow, visCursor := m.barViewport(window, frameBoxed, row2Cols[0].W, row2Cols[0].H)
		bar := renderSTWBarChart(visWindow, visCursor, frameBoxed, m.stwLabelsMode, m.stwTh, 0, row2Cols[0].H, row2Cols[0].W)
		details := renderCycleDetails(visWindow, visCursor, frameBoxed, m.stwTh, row2Cols[1].W, row2Cols[1].H)
		heap := renderHeapLiveHistory(heapHist, frameBoxed, chartView{XSpan: m.xSpan.duration(), YZoomSteps: m.heapYZoom, YPanSteps: m.heapYPan, Focused: m.chartFocus == chartHeap}, row2Cols[2].W, row2Cols[2].H)

		parts = append(parts,
			lipgloss.JoinHorizontal(lipgloss.Top, bar, strings.Repeat(" ", gapX), details, strings.Repeat(" ", gapX), heap),
		)
	}

	if len(rows) >= 3 {
		stw := renderSTWPercentilesHistory(p50Hist, p99Hist, maxHist, frameBoxed, chartView{XSpan: m.xSpan.duration(), YZoomSteps: m.stwYZoom, YPanSteps: m.stwYPan, Focused: m.chartFocus == chartSTW}, rows[2].W, rows[2].H)
		parts = append(parts, stw)
	}

	app := strings.Join(parts, strings.Repeat("\n", gapY))
	app = m.withFooter(app, content.W)
	app = fitViewport(app, content.W, content.H)

	_ = screen
	return lipgloss.NewStyle().Padding(paddingY, paddingX).Render(app)
}

func (m Model) viewTight() string {
	const (
		paddingX = 0
		paddingY = 0
		gridSepX = 1
		gridSepY = 1
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

	// Reserve one line for the footer so panel borders don't get truncated.
	footerH := 1
	contentPanels := content
	if contentPanels.H > footerH {
		contentPanels.H -= footerH
	} else {
		contentPanels.H = 0
	}

	// Fallback for narrow terminals: stack panels vertically.
	if content.W < 90 {
		gridW := contentPanels.W
		gridH := contentPanels.H
		cellW := gridW - 2
		if cellW < 1 || gridH < 3 {
			return lipgloss.NewStyle().Padding(paddingY, paddingX).Render("(terminal too small)")
		}

		rows := stackPanels(Rect{W: cellW, H: gridH - 2}, gridSepY, 3, []int{5, 5, 8, 6, 8, 10})
		if len(rows) == 0 {
			return lipgloss.NewStyle().Padding(paddingY, paddingX).Render("(terminal too small)")
		}

		window, agg, heapHist, p50Hist, p99Hist, maxHist := m.displayData()

		current := renderCurrentValues(agg, framePanel, m.stwTh, cellW, rows[0].H)
		gridRows := []gridRow{
			{cellWidths: []int{cellW}, height: rows[0].H, cells: []string{current}},
		}

		if len(rows) > 1 {
			info := renderInformation(window, agg, m.now, m.lastUpdate, m.snapshotDir, m.lastSnapshot, framePanel, m.stwTh, m.targetEnv, cellW, rows[1].H)
			gridRows = append(gridRows, gridRow{cellWidths: []int{cellW}, height: rows[1].H, cells: []string{info}})
		}

		var visWindow []domain.GCEvent
		visCursor := 0
		if len(rows) > 2 {
			visWindow, visCursor = m.barViewport(window, framePanel, cellW, rows[2].H)
			bar := renderSTWBarChart(visWindow, visCursor, framePanel, m.stwLabelsMode, m.stwTh, 0, rows[2].H, cellW)
			gridRows = append(gridRows, gridRow{cellWidths: []int{cellW}, height: rows[2].H, cells: []string{bar}})
		}
		if len(rows) > 3 {
			details := renderCycleDetails(visWindow, visCursor, framePanel, m.stwTh, cellW, rows[3].H)
			gridRows = append(gridRows, gridRow{cellWidths: []int{cellW}, height: rows[3].H, cells: []string{details}})
		}
		if len(rows) > 4 {
			heap := renderHeapLiveHistory(heapHist, framePanel, chartView{XSpan: m.xSpan.duration(), YZoomSteps: m.heapYZoom, YPanSteps: m.heapYPan, Focused: m.chartFocus == chartHeap}, cellW, rows[4].H)
			gridRows = append(gridRows, gridRow{cellWidths: []int{cellW}, height: rows[4].H, cells: []string{heap}})
		}
		if len(rows) > 5 {
			stw := renderSTWPercentilesHistory(p50Hist, p99Hist, maxHist, framePanel, chartView{XSpan: m.xSpan.duration(), YZoomSteps: m.stwYZoom, YPanSteps: m.stwYPan, Focused: m.chartFocus == chartSTW}, cellW, rows[5].H)
			gridRows = append(gridRows, gridRow{cellWidths: []int{cellW}, height: rows[5].H, cells: []string{stw}})
		}

		app := renderSharedBorderGrid(gridRows, gridW)
		app = m.withFooter(app, content.W)
		app = fitViewport(app, content.W, content.H)
		_ = screen
		return lipgloss.NewStyle().Padding(paddingY, paddingX).Render(app)
	}

	// Height-based layout: scale rows to fit available height.
	// Priorities: row1 (current+info) > row2 (stw+heap) > row3 (stw p50/p99).
	gridW := contentPanels.W
	gridH := contentPanels.H
	if gridW < 10 || gridH < 6 {
		return lipgloss.NewStyle().Padding(paddingY, paddingX).Render("(terminal too small)")
	}
	rows := stackPanels(Rect{W: gridW, H: gridH - 2}, gridSepY, 4, []int{6, 10, 8})
	if len(rows) == 0 {
		return lipgloss.NewStyle().Padding(paddingY, paddingX).Render("(terminal too small)")
	}

	row1AvailW := rows[0].W - 2 - gridSepX
	if row1AvailW < 0 {
		row1AvailW = 0
	}
	row1Cols := Cols(Rect{W: row1AvailW, H: rows[0].H}, 0.50, 0.50)

	window, agg, heapHist, p50Hist, p99Hist, maxHist := m.displayData()

	current := renderCurrentValues(agg, framePanel, m.stwTh, row1Cols[0].W, row1Cols[0].H)
	info := renderInformation(window, agg, m.now, m.lastUpdate, m.snapshotDir, m.lastSnapshot, framePanel, m.stwTh, m.targetEnv, row1Cols[1].W, row1Cols[1].H)

	gridRows := []gridRow{
		{cellWidths: []int{row1Cols[0].W, row1Cols[1].W}, height: rows[0].H, cells: []string{current, info}},
	}

	if len(rows) >= 2 {
		row2AvailW := rows[1].W - 2 - gridSepX*2
		if row2AvailW < 0 {
			row2AvailW = 0
		}
		// Give the bar chart and details more room; heap chart is still readable at ~40%.
		row2Cols := Cols(Rect{W: row2AvailW, H: rows[1].H}, 0.36, 0.24, 0.40)

		visWindow, visCursor := m.barViewport(window, framePanel, row2Cols[0].W, row2Cols[0].H)
		bar := renderSTWBarChart(visWindow, visCursor, framePanel, m.stwLabelsMode, m.stwTh, 0, row2Cols[0].H, row2Cols[0].W)
		details := renderCycleDetails(visWindow, visCursor, framePanel, m.stwTh, row2Cols[1].W, row2Cols[1].H)
		heap := renderHeapLiveHistory(heapHist, framePanel, chartView{XSpan: m.xSpan.duration(), YZoomSteps: m.heapYZoom, YPanSteps: m.heapYPan, Focused: m.chartFocus == chartHeap}, row2Cols[2].W, row2Cols[2].H)

		gridRows = append(gridRows, gridRow{
			cellWidths: []int{row2Cols[0].W, row2Cols[1].W, row2Cols[2].W},
			height:     rows[1].H,
			cells:      []string{bar, details, heap},
		})
	}

	if len(rows) >= 3 {
		cellW := rows[2].W - 2
		if cellW < 1 {
			cellW = 1
		}
		stw := renderSTWPercentilesHistory(p50Hist, p99Hist, maxHist, framePanel, chartView{XSpan: m.xSpan.duration(), YZoomSteps: m.stwYZoom, YPanSteps: m.stwYPan, Focused: m.chartFocus == chartSTW}, cellW, rows[2].H)
		gridRows = append(gridRows, gridRow{cellWidths: []int{cellW}, height: rows[2].H, cells: []string{stw}})
	}

	app := renderSharedBorderGrid(gridRows, gridW)
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
	m.stwMaxHist = appendLimited(m.stwMaxHist, historyPoint{At: at, Value: float64(m.agg.Window.STWMaxUs)}, limit)
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
		m.pausedSTWMax = nil
		return
	}

	m.paused = true
	m.pausedWindow = m.store.Recent()
	m.pausedAgg = domain.ComputeAggregates(m.pausedWindow)
	m.pausedHeapHist = append([]historyPoint(nil), m.heapHistory...)
	m.pausedSTWP50 = append([]historyPoint(nil), m.stwP50Hist...)
	m.pausedSTWP99 = append([]historyPoint(nil), m.stwP99Hist...)
	m.pausedSTWMax = append([]historyPoint(nil), m.stwMaxHist...)
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

func (m *Model) displayData() ([]domain.GCEvent, domain.Aggregates, []historyPoint, []historyPoint, []historyPoint, []historyPoint) {
	if m.paused {
		return m.pausedWindow, m.pausedAgg, m.pausedHeapHist, m.pausedSTWP50, m.pausedSTWP99, m.pausedSTWMax
	}
	return m.store.Recent(), m.agg, m.heapHistory, m.stwP50Hist, m.stwP99Hist, m.stwMaxHist
}

func (m *Model) barViewport(window []domain.GCEvent, frame frameMode, w, h int) ([]domain.GCEvent, int) {
	inner := InnerRect(frameStyle(frame), Rect{W: w, H: h})
	_, _, maxBars := stwBarsCapacity(inner.W)
	if maxBars < 1 {
		maxBars = 1
	}
	if len(window) == 0 {
		return nil, 0
	}

	// In LIVE, cursor tracks last element; in PAUSED it is in absolute window coords (m.cursor).
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

	if len(window) <= maxBars {
		return window, cursorAbs
	}

	// In LIVE we always show the latest bars. In PAUSED we allow paging by selecting a window
	// around the cursor position.
	if !m.paused {
		start := len(window) - maxBars
		return window[start:], maxBars - 1
	}

	start := cursorAbs - maxBars/2
	if start < 0 {
		start = 0
	}
	if start > len(window)-maxBars {
		start = len(window) - maxBars
	}

	vis := window[start : start+maxBars]
	return vis, cursorAbs - start
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

func (m *Model) clampZoomState() {
	var heapHist []historyPoint
	var p50Hist []historyPoint
	var p99Hist []historyPoint
	var maxHist []historyPoint
	if m.paused {
		heapHist = m.pausedHeapHist
		p50Hist = m.pausedSTWP50
		p99Hist = m.pausedSTWP99
		maxHist = m.pausedSTWMax
	} else {
		heapHist = m.heapHistory
		p50Hist = m.stwP50Hist
		p99Hist = m.stwP99Hist
		maxHist = m.stwMaxHist
	}

	xSpan := m.xSpan.duration()
	m.heapYPan = clampPanSteps(heapHist, xSpan, m.heapYZoom, m.heapYPan, 1.0)

	all := make([]historyPoint, 0, len(p50Hist)+len(p99Hist)+len(maxHist))
	all = append(all, p50Hist...)
	all = append(all, p99Hist...)
	all = append(all, maxHist...)
	m.stwYPan = clampPanSteps(all, xSpan, m.stwYZoom, m.stwYPan, 10.0)
}

func clampPanSteps(points []historyPoint, xSpan time.Duration, zoomSteps int, panSteps int, minRange float64) int {
	if len(points) == 0 {
		return 0
	}

	vis := points
	if xSpan > 0 {
		end := points[len(points)-1].At
		start := end.Add(-xSpan)
		idx := 0
		for i, p := range points {
			if !p.At.Before(start) {
				idx = i
				break
			}
		}
		if idx < len(points) {
			vis = points[idx:]
		}
	}
	if len(vis) == 0 {
		return 0
	}

	minY := vis[0].Value
	maxY := vis[0].Value
	for i := 1; i < len(vis); i++ {
		v := vis[i].Value
		if v < minY {
			minY = v
		}
		if v > maxY {
			maxY = v
		}
	}
	if minY < 0 {
		minY = 0
	}
	if maxY < minY {
		maxY = minY
	}

	rng := maxY - minY
	if rng < minRange {
		rng = minRange
	}

	pad := rng * 0.05
	if pad < minRange*0.05 {
		pad = minRange * 0.05
	}
	boundMin := minY - pad
	if boundMin < 0 {
		boundMin = 0
	}
	boundMax := maxY + pad
	if boundMax-boundMin < minRange {
		boundMax = boundMin + minRange
	}

	zoom := zoomSteps
	if zoom < 0 {
		zoom = 0
	}
	if zoom > 0 {
		scale := math.Pow(0.75, float64(zoom))
		if scale < 0.05 {
			scale = 0.05
		}
		rng = rng * scale
		if rng < minRange {
			rng = minRange
		}
	}

	center := (minY + maxY) / 2
	if len(vis) > 0 {
		center = vis[len(vis)-1].Value
	}
	minV0 := center - rng/2
	maxV0 := center + rng/2

	step := rng * 0.2
	if step < minRange*0.2 {
		step = minRange * 0.2
	}
	if step <= 0 {
		return 0
	}

	// We want [minV0 + s, maxV0 + s] to be inside [boundMin, boundMax].
	minShift := boundMin - minV0
	maxShift := boundMax - maxV0

	minSteps := int(math.Ceil(minShift / step))
	maxSteps := int(math.Floor(maxShift / step))
	if minSteps > maxSteps {
		return 0
	}
	if panSteps < minSteps {
		return minSteps
	}
	if panSteps > maxSteps {
		return maxSteps
	}
	return panSteps
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
