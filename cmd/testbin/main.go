package main

import (
	"flag"
	"math/rand"
	"os"
	"runtime"
	"time"

	lab "github.com/timur-developer/gcviz/internal/source/lab"
)

func main() {
	workload := flag.String("workload", lab.PresetAlloc, "Workload preset: "+lab.AvailablePresetsString())
	flag.Parse()

	if !lab.IsValidPreset(*workload) {
		_, _ = os.Stderr.WriteString("unknown workload: " + *workload + "\n")
		_, _ = os.Stderr.WriteString("available: " + lab.AvailablePresetsString() + "\n")
		os.Exit(2)
	}

	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))

	switch *workload {
	case lab.PresetAlloc:
		generateAllocLoad(rnd)
	case lab.PresetChurn:
		generateChurnLoad(rnd)
	case lab.PresetIdle:
		generateIdleLoad(rnd)
	case lab.PresetSpike:
		generateSpikeLoad(rnd)
	default:
		os.Exit(2)
	}
}

func generateAllocLoad(rnd *rand.Rand) {
	var keep [][]byte

	ticker := time.NewTicker(40 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		n := 8 + rnd.Intn(16)
		for i := 0; i < n; i++ {
			b := make([]byte, 32*1024+rnd.Intn(192*1024))
			b[0] = byte(rnd.Intn(256))
			keep = append(keep, b)
		}

		if len(keep) > 512 {
			keep = keep[len(keep)-256:]
		}
	}
}

func generateChurnLoad(rnd *rand.Rand) {
	ticker := time.NewTicker(60 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		n := 1 + rnd.Intn(4)
		hot := make([][]byte, 0, n)
		for i := 0; i < n; i++ {
			b := make([]byte, 4*1024*1024+rnd.Intn(6*1024*1024))
			b[0] = byte(rnd.Intn(256))
			hot = append(hot, b)
		}

		// keep the burst alive briefly so it survives at least one GC
		time.Sleep(25 * time.Millisecond)
		runtime.KeepAlive(hot)
	}
}

func generateIdleLoad(rnd *rand.Rand) {
	// Mostly idle, but with occasional small bursts to guarantee GC activity
	// (UI updates happen on gctrace events).
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	nextBurst := time.Now().Add(2*time.Second + time.Duration(rnd.Intn(1000))*time.Millisecond)
	for range ticker.C {
		if time.Now().Before(nextBurst) {
			continue
		}

		// short burst: some medium allocations, don't retain much
		n := 40 + rnd.Intn(40)
		for i := 0; i < n; i++ {
			b := make([]byte, 256*1024+rnd.Intn(256*1024))
			b[0] = byte(rnd.Intn(256))
		}

		nextBurst = time.Now().Add(2*time.Second + time.Duration(rnd.Intn(1500))*time.Millisecond)
	}
}

func generateSpikeLoad(rnd *rand.Rand) {
	var keep [][]byte

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	nextSpike := time.Now()

	for range ticker.C {
		now := time.Now()
		if now.After(nextSpike) {
			// heavy wave
			for i := 0; i < 320; i++ {
				b := make([]byte, 256*1024+rnd.Intn(1024*1024))
				b[0] = byte(rnd.Intn(256))
				keep = append(keep, b)
			}
			// spikes should be frequent enough to be visible in the UI
			nextSpike = now.Add(900*time.Millisecond + time.Duration(rnd.Intn(900))*time.Millisecond)
		} else {
			// light background traffic between spikes
			n := 6 + rnd.Intn(12)
			for i := 0; i < n; i++ {
				b := make([]byte, 64*1024+rnd.Intn(256*1024))
				b[0] = byte(rnd.Intn(256))
				keep = append(keep, b)
			}
		}

		if len(keep) > 1024 {
			keep = keep[len(keep)-512:]
		}
	}
}
