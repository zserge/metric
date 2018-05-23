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
	c := &counter{}
	assertJSON(t, c, h{"type": "c", "count": 0})
	c.Add(1)
	assertJSON(t, c, h{"type": "c", "count": 1})
	c.Add(10)
	assertJSON(t, c, h{"type": "c", "count": 11})
	c.Reset()
	assertJSON(t, c, h{"type": "c", "count": 0})
}

func TestGauge(t *testing.T) {
	g := &gauge{}
	assertJSON(t, g, h{"type": "g", "mean": 0, "min": 0, "max": 0})
	g.Add(1)
	assertJSON(t, g, h{"type": "g", "mean": 1, "min": 1, "max": 1})
	g.Add(5)
	assertJSON(t, g, h{"type": "g", "mean": 3, "min": 1, "max": 5})
	g.Add(0)
	assertJSON(t, g, h{"type": "g", "mean": 2, "min": 0, "max": 5})
	g.Reset()
	assertJSON(t, g, h{"type": "g", "mean": 0, "min": 0, "max": 0})
}

func TestHistogram(t *testing.T) {
	hist := &histogram{}
	assertJSON(t, hist, h{"type": "h", "p50": 0, "p90": 0, "p99": 0})
	hist.Add(1)
	assertJSON(t, hist, h{"type": "h", "p50": 1, "p90": 1, "p99": 1})
	hist.Reset()
	for i := 0; i < 100; i++ {
		hist.Add(float64(i))
	}
	assertJSON(t, hist, h{"type": "h", "p50": 49, "p90": 89, "p99": 98})
}

func TestHistogramNormalDist(t *testing.T) {
	hist := &histogram{}
	rand.Seed(time.Now().UnixNano())
	for i := 0; i < 10000; i++ {
		hist.Add(rand.Float64() * 10)
	}
	b, _ := hist.MarshalJSON()
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

func TestTimeline(t *testing.T) {
	now = mockTime(0)
	c := NewCounter("3s1s")
	count := func(x float64) h { return h{"type": "c", "count": x} }
	assertJSON(t, c, h{"interval": 1, "samples": v{count(0), count(0), count(0)}})
	c.Add(1)
	assertJSON(t, c, h{"interval": 1, "samples": v{count(1), count(0), count(0)}})
	now = mockTime(1)
	assertJSON(t, c, h{"interval": 1, "samples": v{count(0), count(1), count(0)}})
	c.Add(5)
	assertJSON(t, c, h{"interval": 1, "samples": v{count(5), count(1), count(0)}})
	now = mockTime(3)
	assertJSON(t, c, h{"interval": 1, "samples": v{count(0), count(0), count(5)}})
}

func TestExpVar(t *testing.T) {
	now = mockTime(0)
	expvar.Publish("test:count", NewCounter())
	expvar.Publish("test:timeline", NewCounter("3s1s"))
	expvar.Get("test:count").(Metric).Add(1)
	expvar.Get("test:timeline").(Metric).Add(1)
	if expvar.Get("test:count").String() != `{"type":"c","count":1}` {
		t.Fatal(expvar.Get("test:count"))
	}
	if expvar.Get("test:timeline").String() != `{"interval":1,"samples":[{"type":"c","count":1},{"type":"c","count":0},{"type":"c","count":0}]}` {
		t.Fatal(expvar.Get("test:timeline"))
	}
	now = mockTime(1)
	if expvar.Get("test:count").String() != `{"type":"c","count":1}` {
		t.Fatal(expvar.Get("test:count"))
	}
	if expvar.Get("test:timeline").String() != `{"interval":1,"samples":[{"type":"c","count":0},{"type":"c","count":1},{"type":"c","count":0}]}` {
		t.Fatal(expvar.Get("test:timeline"))
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
