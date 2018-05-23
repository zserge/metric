package metric

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// To mock time in tests
var now = time.Now

type Metric interface {
	Add(n float64)
	Reset()
	String() string
}

func NewCounter(frames ...string) Metric {
	return newMetric(func() Metric { return &counter{} }, frames...)
}

func NewGauge(frames ...string) Metric {
	return newMetric(func() Metric { return &gauge{} }, frames...)
}

func NewHistogram(frames ...string) Metric {
	return newMetric(func() Metric { return &histogram{} }, frames...)
}

type timeseries struct {
	sync.Mutex
	now      time.Time
	size     int
	interval time.Duration
	samples  []Metric
}

func (ts *timeseries) Reset() {
	for _, s := range ts.samples {
		s.Reset()
	}
}

func (ts *timeseries) roll() {
	t := now()
	roll := int((t.Round(ts.interval).Sub(ts.now.Round(ts.interval))) / ts.interval)
	ts.now = t
	n := len(ts.samples)
	if roll <= 0 {
		return
	}
	if roll >= len(ts.samples) {
		ts.Reset()
	} else {
		for i := 0; i < roll; i++ {
			tmp := ts.samples[n-1]
			for j := n - 1; j > 0; j-- {
				ts.samples[j] = ts.samples[j-1]
			}
			ts.samples[0] = tmp
			ts.samples[0].Reset()
		}
	}
}

func (ts *timeseries) Add(n float64) {
	ts.Lock()
	defer ts.Unlock()
	ts.roll()
	ts.samples[0].Add(n)
}

func (ts *timeseries) MarshalJSON() ([]byte, error) {
	ts.Lock()
	defer ts.Unlock()
	ts.roll()
	return json.Marshal(ts.samples)
}

func (ts *timeseries) String() string {
	b, _ := ts.MarshalJSON()
	return string(b)
}

type multimetric []Metric

func (mm multimetric) Add(n float64) {
	for _, m := range mm {
		m.Add(n)
	}
}
func (mm multimetric) Reset() {
	for _, m := range mm {
		m.Reset()
	}
}
func (mm multimetric) String() string {
	s := "{"
	for i, m := range mm {
		if i != 0 {
			s = s + ","
		}
		s = s + m.String()
	}
	s = s + "}"
	return s
}

func strjson(x interface{}) string {
	b, _ := json.Marshal(x)
	return string(b)
}

type counter struct {
	count float64
}

func (c *counter) String() string { return strjson(c) }
func (c *counter) Reset()         { c.count = 0 }
func (c *counter) Add(n float64)  { c.count += n }
func (c *counter) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Count float64 `json:"count"`
	}{c.count})
}

type gauge struct {
	sum   float64
	min   float64
	max   float64
	count int
}

func (g *gauge) String() string { return strjson(g) }
func (g *gauge) Reset()         { g.count, g.sum, g.min, g.max = 0, 0, 0, 0 }
func (g *gauge) Add(n float64) {
	if n < g.min || g.count == 0 {
		g.min = n
	}
	if n > g.max || g.count == 0 {
		g.max = n
	}
	g.sum += n
	g.count++
}

func (g *gauge) MarshalJSON() ([]byte, error) {
	mean := g.sum / float64(g.count)
	if g.count == 0 {
		mean = 0
	}
	return json.Marshal(struct {
		Mean float64 `json:"mean"`
		Min  float64 `json:"min"`
		Max  float64 `json:"max"`
	}{mean, g.min, g.max})
}

const maxBins = 100

type bin struct {
	value float64
	count float64
}

type histogram struct {
	bins  []bin
	total uint64
}

func (h *histogram) String() string { return strjson(h) }
func (h *histogram) Reset()         { h.bins = nil; h.total = 0 }

func (h *histogram) Add(n float64) {
	defer h.trim()
	h.total++
	for i := range h.bins {
		if h.bins[i].value == n {
			h.bins[i].count++
			return
		}
		if h.bins[i].value > n {
			newbin := bin{value: n, count: 1}
			head := append(make([]bin, 0), h.bins[0:i]...)
			head = append(head, newbin)
			tail := h.bins[i:]
			h.bins = append(head, tail...)
			return
		}
	}

	h.bins = append(h.bins, bin{count: 1, value: n})
}

func (h *histogram) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		P50 float64 `json:"p50"`
		P90 float64 `json:"p90"`
		P99 float64 `json:"p99"`
	}{h.quantile(0.5), h.quantile(0.9), h.quantile(0.99)})
}

func (h *histogram) trim() {
	for len(h.bins) > maxBins {
		minDelta := 1e99
		minDeltaIndex := 0
		for i := range h.bins {
			if i == 0 {
				continue
			}
			if delta := h.bins[i].value - h.bins[i-1].value; delta < minDelta {
				minDelta = delta
				minDeltaIndex = i
			}
		}
		totalCount := h.bins[minDeltaIndex-1].count + h.bins[minDeltaIndex].count
		mergedbin := bin{
			value: (h.bins[minDeltaIndex-1].value*
				h.bins[minDeltaIndex-1].count +
				h.bins[minDeltaIndex].value*
					h.bins[minDeltaIndex].count) /
				totalCount, // weighted average
			count: totalCount, // summed heights
		}
		head := append(make([]bin, 0), h.bins[0:minDeltaIndex-1]...)
		tail := append([]bin{mergedbin}, h.bins[minDeltaIndex+1:]...)
		h.bins = append(head, tail...)
	}
}

func (h *histogram) quantile(q float64) float64 {
	count := q * float64(h.total)
	for i := range h.bins {
		count -= float64(h.bins[i].count)
		if count <= 0 {
			return h.bins[i].value
		}
	}
	return 0
}

func newTimeseries(builder func() Metric, frame string) *timeseries {
	var (
		totalNum, intervalNum   int
		totalUnit, intervalUnit rune
	)
	units := map[rune]time.Duration{
		's': time.Second,
		'm': time.Minute,
		'h': time.Hour,
		'd': time.Hour * 24,
		'w': time.Hour * 24 * 7,
		'M': time.Hour * 24 * 7 * 30,
		'y': time.Hour * 24 * 7 * 365,
	}
	fmt.Sscanf(frame, "%d%c%d%c", &totalNum, &totalUnit, &intervalNum, &intervalUnit)
	interval := units[intervalUnit] * time.Duration(intervalNum)
	if interval == 0 {
		interval = time.Minute
	}
	totalDuration := units[totalUnit] * time.Duration(totalNum)
	if totalDuration == 0 {
		totalDuration = interval * 15
	}
	n := int(totalDuration / interval)
	samples := make([]Metric, n, n)
	for i := 0; i < n; i++ {
		samples[i] = builder()
	}
	return &timeseries{interval: interval, samples: samples}
}

func newMetric(builder func() Metric, frames ...string) Metric {
	if len(frames) == 0 {
		return builder()
	}
	if len(frames) == 1 {
		return newTimeseries(builder, frames[0])
	}
	mm := multimetric{}
	for _, frame := range frames {
		mm = append(mm, newTimeseries(builder, frame))
	}
	return mm
}
