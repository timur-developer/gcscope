package domain

type GCEvent struct {
	GCNum            int
	TimeSinceStartS  float64
	GCCPUPercent     float64
	STWSweepTermMs   float64
	MarkMs           float64
	STWMarkTermMs    float64
	HeapStartMB      int
	HeapEndMB        int
	HeapLiveMB       int
	HeapGoalMB       int
	NumP             int
	Forced           bool
	SweepHeapSizeMB  int
	PagesSwept       int
	AssistRatio      float64
	AssistWorkers    int
	CPUPercent       int
	ConsMark         float64
}
