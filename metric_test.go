package metric

import (
	"encoding/json"
	"expvar"
	"math"
	"math/rand"
	"reflect"
	"testing"
	"time"
)

type (
	h map[string]interface{}
	v []interface{}
)

func mockTime(sec int) func() time.Time {
	return func() time.Time {
		return time.Date(2017, 8, 11, 9, 0, sec, 0, time.UTC)
	}
}

func assertJSON(t *testing.T, o1, o2 interface{}) {
	var result, expect interface{}
	if reflect.TypeOf(o2).Kind() == reflect.Slice {
		result, expect = v{}, v{}
	} else {
		result, expect = h{}, h{}
	}
	if b1, err := json.Marshal(o1); err != nil {
		t.Fatal(o1, err)
	} else if err := json.Unmarshal(b1, &result); err != nil {
		t.Fatal(err)
	} else if b2, err := json.Marshal(o2); err != nil {
		t.Fatal(o2, err)
	} else if err := json.Unmarshal(b2, &expect); err != nil {
		t.Fatal(err)
	} else if !reflect.DeepEqual(result, expect) {
		t.Fatal(result, expect)
	}
}

func TestCounter(t *testing.T) {
	c := NewCounter()
	assertJSON(t, c, h{"type": "c", "count": 0})
	c.Add(1)
	assertJSON(t, c, h{"type": "c", "count": 1})
	c.Add(10)
	assertJSON(t, c, h{"type": "c", "count": 11})
}

func TestGauge(t *testing.T) {
	g := NewGauge()
	assertJSON(t, g, h{"type": "g", "mean": 0, "min": 0, "max": 0, "value": 0})
	g.Add(1)
	assertJSON(t, g, h{"type": "g", "mean": 1, "min": 1, "max": 1, "value": 1})
	g.Add(5)
	assertJSON(t, g, h{"type": "g", "mean": 3, "min": 1, "max": 5, "value": 5})
	g.Add(0)
	assertJSON(t, g, h{"type": "g", "mean": 2, "min": 0, "max": 5, "value": 0})
}

func TestHistogram(t *testing.T) {
	hist := NewHistogram()
	assertJSON(t, hist, h{"type": "h", "p50": 0, "p90": 0, "p99": 0})
	hist.Add(1)
	assertJSON(t, hist, h{"type": "h", "p50": 1, "p90": 1, "p99": 1})
	for i := 2; i < 100; i++ {
		hist.Add(float64(i))
	}
	assertJSON(t, hist, h{"type": "h", "p50": 50, "p90": 90, "p99": 99})
}

func TestHistogramNormalDist(t *testing.T) {
	hist := NewHistogram()
	rand.Seed(time.Now().UnixNano())
	for i := 0; i < 10000; i++ {
		hist.Add(rand.Float64() * 10)
	}
	b, _ := json.Marshal(hist)
	p := h{}
	json.Unmarshal(b, &p)
	if math.Abs(p["p50"].(float64)-5) > 0.5 {
		t.Fatal(p["p50"])
	}
	if math.Abs(p["p90"].(float64)-9) > 0.5 {
		t.Fatal(p["p90"])
	}
	if math.Abs(p["p99"].(float64)-10) > 0.5 {
		t.Fatal(p["p99"])
	}
}

func TestMetricReset(t *testing.T) {
	c := &counter{}
	c.Add(5)
	assertJSON(t, c, h{"type": "c", "count": 5})
	c.Reset()
	assertJSON(t, c, h{"type": "c", "count": 0})

	g := &gauge{}
	g.Add(5)
	assertJSON(t, g, h{"type": "g", "mean": 5, "min": 5, "max": 5, "value": 5})
	g.Reset()
	assertJSON(t, g, h{"type": "g", "mean": 0, "min": 0, "max": 0, "value": 0})

	hist := &histogram{}
	hist.Add(5)
	assertJSON(t, hist, h{"type": "h", "p50": 5, "p90": 5, "p99": 5})
	hist.Reset()
	assertJSON(t, hist, h{"type": "h", "p50": 0, "p90": 0, "p99": 0})
}

func TestMetricString(t *testing.T) {
	c := NewCounter()
	c.Add(1)
	c.Add(3)
	if s := c.String(); s != "4" {
		t.Fatal(s)
	}

	g := NewGauge()
	g.Add(1)
	g.Add(3)
	if s := g.String(); s != "3" {
		t.Fatal(s)
	}

	hist := NewHistogram()
	hist.Add(1)
	hist.Add(3)
	if s := hist.String(); s != `{"p50":1,"p90":3,"p99":3}` {
		t.Fatal(s)
	}
}

func TestCounterTimeline(t *testing.T) {
	now = mockTime(0)
	c := NewCounter("3s1s")
	expect := func(total float64, samples ...float64) h {
		timeline := v{}
		for _, s := range samples {
			timeline = append(timeline, h{"type": "c", "count": s})
		}
		return h{
			"interval": 1,
			"total":    h{"type": "c", "count": total},
			"samples":  timeline,
		}
	}
	assertJSON(t, c, expect(0, 0, 0, 0))
	c.Add(1)
	assertJSON(t, c, expect(1, 1, 0, 0))
	now = mockTime(1)
	assertJSON(t, c, expect(1, 0, 1, 0))
	c.Add(5)
	assertJSON(t, c, expect(6, 5, 1, 0))
	now = mockTime(3)
	assertJSON(t, c, expect(5, 0, 0, 5))
	now = mockTime(10)
	assertJSON(t, c, expect(0, 0, 0, 0))
}

func TestGaugeTimeline(t *testing.T) {
	now = mockTime(0)
	g := NewGauge("3s1s")
	gauge := func(value, min, max, mean float64) h {
		return h{"type": "g", "value": value, "min": min, "max": max, "mean": mean}
	}
	expect := func(total h, samples ...h) h {
		return h{"interval": 1, "total": total, "samples": samples}
	}
	assertJSON(t, g, expect(gauge(0, 0, 0, 0), gauge(0, 0, 0, 0), gauge(0, 0, 0, 0), gauge(0, 0, 0, 0)))
	g.Add(1)
	assertJSON(t, g, expect(gauge(1, 1, 1, 1), gauge(1, 1, 1, 1), gauge(0, 0, 0, 0), gauge(0, 0, 0, 0)))
	now = mockTime(1)
	assertJSON(t, g, expect(gauge(1, 1, 1, 1), gauge(0, 0, 0, 0), gauge(1, 1, 1, 1), gauge(0, 0, 0, 0)))
	g.Add(5)
	assertJSON(t, g, expect(gauge(5, 1, 5, 3), gauge(5, 5, 5, 5), gauge(1, 1, 1, 1), gauge(0, 0, 0, 0)))
	now = mockTime(3)
	assertJSON(t, g, expect(gauge(5, 5, 5, 5), gauge(0, 0, 0, 0), gauge(0, 0, 0, 0), gauge(5, 5, 5, 5)))
	now = mockTime(10)
	assertJSON(t, g, expect(gauge(0, 0, 0, 0), gauge(0, 0, 0, 0), gauge(0, 0, 0, 0), gauge(0, 0, 0, 0)))
}

func TestHistogramTimeline(t *testing.T) {
	now = mockTime(0)
	hist := NewHistogram("3s1s")
	histogram := func(p50, p90, p99 float64) h {
		return h{"type": "h", "p50": p50, "p90": p90, "p99": p99}
	}
	expect := func(total h, samples ...h) h {
		return h{"interval": 1, "total": total, "samples": samples}
	}
	assertJSON(t, hist, expect(histogram(0, 0, 0), histogram(0, 0, 0), histogram(0, 0, 0), histogram(0, 0, 0)))
	hist.Add(1)
	assertJSON(t, hist, expect(histogram(1, 1, 1), histogram(1, 1, 1), histogram(0, 0, 0), histogram(0, 0, 0)))
	now = mockTime(1)
	assertJSON(t, hist, expect(histogram(1, 1, 1), histogram(0, 0, 0), histogram(1, 1, 1), histogram(0, 0, 0)))
	hist.Add(3)
	hist.Add(5)
	assertJSON(t, hist, expect(histogram(3, 5, 5), histogram(3, 5, 5), histogram(1, 1, 1), histogram(0, 0, 0)))
	now = mockTime(3)
	assertJSON(t, hist, expect(histogram(3, 5, 5), histogram(0, 0, 0), histogram(0, 0, 0), histogram(3, 5, 5)))
	now = mockTime(10)
	assertJSON(t, hist, expect(histogram(0, 0, 0), histogram(0, 0, 0), histogram(0, 0, 0), histogram(0, 0, 0)))
}

func TestMulti(t *testing.T) {
	m := NewCounter("10s1s", "30s5s")
	m.Add(5)
	if s := m.String(); s != `5` {
		t.Fatal(s)
	}
}

func TestExpVar(t *testing.T) {
	expvar.Publish("test:count", NewCounter())
	expvar.Publish("test:timeline", NewGauge("3s1s"))
	expvar.Get("test:count").(Metric).Add(1)
	expvar.Get("test:timeline").(Metric).Add(1)
	if s := expvar.Get("test:count").String(); s != `1` {
		t.Fatal(s)
	}
	if s := expvar.Get("test:timeline").String(); s != `1` {
		t.Fatal(s)
	}
}

func BenchmarkMetrics(b *testing.B) {
	b.Run("counter", func(b *testing.B) {
		c := &counter{}
		for i := 0; i < b.N; i++ {
			c.Add(rand.Float64())
		}
	})
	b.Run("gauge", func(b *testing.B) {
		c := &gauge{}
		for i := 0; i < b.N; i++ {
			c.Add(rand.Float64())
		}
	})
	b.Run("histogram", func(b *testing.B) {
		c := &histogram{}
		for i := 0; i < b.N; i++ {
			c.Add(rand.Float64())
		}
	})
	b.Run("timeline/counter", func(b *testing.B) {
		c := NewCounter("10s1s")
		for i := 0; i < b.N; i++ {
			c.Add(rand.Float64())
		}
	})
	b.Run("timeline/gauge", func(b *testing.B) {
		c := NewGauge("10s1s")
		for i := 0; i < b.N; i++ {
			c.Add(rand.Float64())
		}
	})
	b.Run("timeline/histogram", func(b *testing.B) {
		c := NewHistogram("10s1s")
		for i := 0; i < b.N; i++ {
			c.Add(rand.Float64())
		}
	})
}
