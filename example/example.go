package main

import (
	"expvar"
	"fmt"
	"math/rand"
	"net/http"
	"runtime"
	"time"

	"github.com/zserge/metric"
)

func fibrec(n int) int {
	if n <= 1 {
		return n
	}
	return fibrec(n-1) + fibrec(n-2)
}

func main() {
	// Fibonacci: how long it takes and how many calls were made
	expvar.Publish("fib:rec:sec", metric.NewHistogram("120s1s", "15m10s", "1h1m"))
	expvar.Publish("fib:rec:count", metric.NewCounter("120s1s", "15m10s", "1h1m"))

	// Random numbers always look nice on graphs
	expvar.Publish("random:gauge", metric.NewGauge("60s1s"))
	expvar.Publish("random:hist", metric.NewHistogram("2m1s", "15m30s", "1h1m"))

	// Some Go internal metrics
	expvar.Publish("go:numgoroutine", metric.NewGauge("2ms1s", "15m30s", "1h1m"))
	expvar.Publish("go:numcgocall", metric.NewGauge("2ms1s", "15m30s", "1h1m"))
	expvar.Publish("go:alloc", metric.NewGauge("2ms1s", "15m30s", "1h1m"))
	expvar.Publish("go:alloctotal", metric.NewGauge("2ms1s", "15m30s", "1h1m"))

	go func() {
		for range time.Tick(123 * time.Millisecond) {
			expvar.Get("random:gauge").(metric.Metric).Add(rand.Float64())
			expvar.Get("random:hist").(metric.Metric).Add(rand.Float64() * 100)
		}
	}()
	go func() {
		for range time.Tick(100 * time.Millisecond) {
			m := &runtime.MemStats{}
			runtime.ReadMemStats(m)
			expvar.Get("go:numgoroutine").(metric.Metric).Add(float64(runtime.NumGoroutine()))
			expvar.Get("go:numcgocall").(metric.Metric).Add(float64(runtime.NumCgoCall()))
			expvar.Get("go:alloc").(metric.Metric).Add(float64(m.Alloc) / 1000000)
			expvar.Get("go:alloctotal").(metric.Metric).Add(float64(m.TotalAlloc) / 1000000)
		}
	}()
	http.Handle("/debug/metrics", metric.Handler(metric.Exposed))
	http.HandleFunc("/fibrec", func(w http.ResponseWriter, r *http.Request) {
		expvar.Get("fib:rec:count").(metric.Metric).Add(1)
		start := time.Now()
		fmt.Fprintf(w, "%d", fibrec(40))
		expvar.Get("fib:rec:sec").(metric.Metric).Add(float64(time.Now().Sub(start)) / float64(time.Second))
	})
	http.ListenAndServe(":8000", nil)
}
