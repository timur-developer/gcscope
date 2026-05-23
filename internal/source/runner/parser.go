package runner

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/timur-developer/gcviz/internal/domain"
)

var (
	ErrParseGCLine     = errors.New("failed to parse gc line")
	ErrParsePacerLine  = errors.New("failed to parse pacer line")
	ErrPacerWithoutGC  = errors.New("pacer line without active gc event")
	gcHeaderRe         = regexp.MustCompile(`^gc\s+(\d+)\s+@([0-9.]+)s\s+([0-9.]+)%:\s+(.+)$`)
	gcNumPRe           = regexp.MustCompile(`(\d+)\s+P(?:\s+\(forced\))?$`)
	numberPattern      = `[-+]?\d+(?:\.\d+)?(?:[eE][-+]?\d+)?`
	pacerSweepRe       = regexp.MustCompile(`^pacer:\s+sweep done at heap size (\d+)MB;.*swept (\d+) pages`)
	pacerAssistRe      = regexp.MustCompile(`^pacer:\s+assist ratio=(` + numberPattern + `) .* workers=(\d+)\+(` + numberPattern + `)`)
	pacerCPURe         = regexp.MustCompile(`^pacer:\s+(\d+)%\s+CPU .* cons/mark (` + numberPattern + `)\)$`)
)

type Parser struct {
	current *domain.GCEvent
}

func NewParser() *Parser {
	return &Parser{}
}

func (p *Parser) ParseLine(line string) (*domain.GCEvent, error) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return nil, nil
	}
	if strings.HasPrefix(trimmed, "gc ") {
		return p.parseGCLine(trimmed)
	}
	if strings.HasPrefix(trimmed, "pacer:") {
		return nil, p.parsePacerLine(trimmed)
	}
	return nil, nil
}

func (p *Parser) Flush() *domain.GCEvent {
	if p.current == nil {
		return nil
	}
	event := p.current
	p.current = nil
	return event
}

func (p *Parser) parseGCLine(line string) (*domain.GCEvent, error) {
	matches := gcHeaderRe.FindStringSubmatch(line)
	if len(matches) != 5 {
		return nil, fmt.Errorf("%w: %q", ErrParseGCLine, line)
	}

	gcNum, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, fmt.Errorf("%w: %q", ErrParseGCLine, line)
	}
	timeSinceStart, err := strconv.ParseFloat(matches[2], 64)
	if err != nil {
		return nil, fmt.Errorf("%w: %q", ErrParseGCLine, line)
	}
	gcCPUPercent, err := strconv.ParseFloat(matches[3], 64)
	if err != nil {
		return nil, fmt.Errorf("%w: %q", ErrParseGCLine, line)
	}

	rest := matches[4]
	clockSplit := strings.SplitN(rest, " ms clock, ", 2)
	if len(clockSplit) != 2 {
		return nil, fmt.Errorf("%w: %q", ErrParseGCLine, line)
	}
	clockParts := strings.Split(clockSplit[0], "+")
	if len(clockParts) != 3 {
		return nil, fmt.Errorf("%w: %q", ErrParseGCLine, line)
	}
	stwSweepTerm, err := strconv.ParseFloat(clockParts[0], 64)
	if err != nil {
		return nil, fmt.Errorf("%w: %q", ErrParseGCLine, line)
	}
	markMs, err := strconv.ParseFloat(clockParts[1], 64)
	if err != nil {
		return nil, fmt.Errorf("%w: %q", ErrParseGCLine, line)
	}
	stwMarkTerm, err := strconv.ParseFloat(clockParts[2], 64)
	if err != nil {
		return nil, fmt.Errorf("%w: %q", ErrParseGCLine, line)
	}

	cpuIdx := strings.Index(clockSplit[1], " ms cpu, ")
	if cpuIdx < 0 {
		return nil, fmt.Errorf("%w: %q", ErrParseGCLine, line)
	}
	rest = clockSplit[1][cpuIdx+len(" ms cpu, "):]

	heapSplit := strings.SplitN(rest, " MB, ", 2)
	if len(heapSplit) != 2 {
		return nil, fmt.Errorf("%w: %q", ErrParseGCLine, line)
	}
	heapParts := strings.Split(heapSplit[0], "->")
	if len(heapParts) != 3 {
		return nil, fmt.Errorf("%w: %q", ErrParseGCLine, line)
	}
	heapStart, err := strconv.Atoi(heapParts[0])
	if err != nil {
		return nil, fmt.Errorf("%w: %q", ErrParseGCLine, line)
	}
	heapEnd, err := strconv.Atoi(heapParts[1])
	if err != nil {
		return nil, fmt.Errorf("%w: %q", ErrParseGCLine, line)
	}
	heapLive, err := strconv.Atoi(heapParts[2])
	if err != nil {
		return nil, fmt.Errorf("%w: %q", ErrParseGCLine, line)
	}

	goalSplit := strings.SplitN(heapSplit[1], " MB goal, ", 2)
	if len(goalSplit) != 2 {
		return nil, fmt.Errorf("%w: %q", ErrParseGCLine, line)
	}
	heapGoal, err := strconv.Atoi(goalSplit[0])
	if err != nil {
		return nil, fmt.Errorf("%w: %q", ErrParseGCLine, line)
	}

	numPMatch := gcNumPRe.FindStringSubmatch(goalSplit[1])
	if len(numPMatch) != 2 {
		return nil, fmt.Errorf("%w: %q", ErrParseGCLine, line)
	}
	numP, err := strconv.Atoi(numPMatch[1])
	if err != nil {
		return nil, fmt.Errorf("%w: %q", ErrParseGCLine, line)
	}
	forced := strings.HasSuffix(strings.TrimSpace(line), "(forced)")

	newEvent := &domain.GCEvent{
		GCNum:           gcNum,
		TimeSinceStartS: timeSinceStart,
		GCCPUPercent:    gcCPUPercent,
		STWSweepTermMs:  stwSweepTerm,
		MarkMs:          markMs,
		STWMarkTermMs:   stwMarkTerm,
		HeapStartMB:     heapStart,
		HeapEndMB:       heapEnd,
		HeapLiveMB:      heapLive,
		HeapGoalMB:      heapGoal,
		NumP:            numP,
		Forced:          forced,
	}

	previous := p.current
	p.current = newEvent
	return previous, nil
}

func (p *Parser) parsePacerLine(line string) error {
	if p.current == nil {
		return fmt.Errorf("%w: %q", ErrPacerWithoutGC, line)
	}

	if matches := pacerSweepRe.FindStringSubmatch(line); len(matches) == 3 {
		heapSize, err := strconv.Atoi(matches[1])
		if err != nil {
			return fmt.Errorf("%w: %q", ErrParsePacerLine, line)
		}
		pages, err := strconv.Atoi(matches[2])
		if err != nil {
			return fmt.Errorf("%w: %q", ErrParsePacerLine, line)
		}
		p.current.SweepHeapSizeMB = heapSize
		p.current.PagesSwept = pages
		return nil
	}

	if matches := pacerAssistRe.FindStringSubmatch(line); len(matches) == 4 {
		ratio, err := strconv.ParseFloat(matches[1], 64)
		if err != nil {
			return fmt.Errorf("%w: %q", ErrParsePacerLine, line)
		}
		workersBase, err := strconv.Atoi(matches[2])
		if err != nil {
			return fmt.Errorf("%w: %q", ErrParsePacerLine, line)
		}
		workersExtra, err := strconv.ParseFloat(matches[3], 64)
		if err != nil {
			return fmt.Errorf("%w: %q", ErrParsePacerLine, line)
		}
		p.current.AssistRatio = ratio
		p.current.AssistWorkers = workersBase + int(workersExtra)
		return nil
	}

	if matches := pacerCPURe.FindStringSubmatch(line); len(matches) == 3 {
		cpuPercent, err := strconv.Atoi(matches[1])
		if err != nil {
			return fmt.Errorf("%w: %q", ErrParsePacerLine, line)
		}
		consMark, err := strconv.ParseFloat(matches[2], 64)
		if err != nil {
			return fmt.Errorf("%w: %q", ErrParsePacerLine, line)
		}
		p.current.CPUPercent = cpuPercent
		p.current.ConsMark = consMark
		return nil
	}

	return fmt.Errorf("%w: %q", ErrParsePacerLine, line)
}
