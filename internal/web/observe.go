package web

import (
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"sync"
	"time"
)

type httpKey struct {
	method string
	status int
}

var httpStats = struct {
	sync.Mutex
	counts map[httpKey]uint64
	sum    float64
	total  uint64
}{counts: map[httpKey]uint64{}}

// WriteHTTPMetrics добавляет метрики HTTP к ответу /metrics.
func WriteHTTPMetrics(w http.ResponseWriter) {
	httpStats.Lock()
	counts := make(map[httpKey]uint64, len(httpStats.counts))
	for k, v := range httpStats.counts {
		counts[k] = v
	}
	sum := httpStats.sum
	total := httpStats.total
	httpStats.Unlock()

	keys := make([]httpKey, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].method == keys[j].method {
			return keys[i].status < keys[j].status
		}
		return keys[i].method < keys[j].method
	})
	_, _ = fmt.Fprintln(w, "# HELP tgadmin_http_requests_total Запросы к HTTP API")
	_, _ = fmt.Fprintln(w, "# TYPE tgadmin_http_requests_total counter")
	for _, k := range keys {
		_, _ = fmt.Fprintf(w, "tgadmin_http_requests_total{method=%q,status=%q} %d\n",
			k.method, fmt.Sprint(k.status), counts[k])
	}
	_, _ = fmt.Fprintln(w, "# HELP tgadmin_http_request_duration_seconds_sum Суммарная длительность запросов")
	_, _ = fmt.Fprintln(w, "# TYPE tgadmin_http_request_duration_seconds_sum counter")
	_, _ = fmt.Fprintf(w, "tgadmin_http_request_duration_seconds_sum %f\n", sum)
	_, _ = fmt.Fprintln(w, "# HELP tgadmin_http_request_duration_seconds_count Количество запросов")
	_, _ = fmt.Fprintln(w, "# TYPE tgadmin_http_request_duration_seconds_count counter")
	_, _ = fmt.Fprintf(w, "tgadmin_http_request_duration_seconds_count %d\n", total)
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)
		dur := time.Since(start)
		status := sw.status
		if status == 0 {
			status = http.StatusOK
		}
		httpStats.Lock()
		httpStats.counts[httpKey{method: r.Method, status: status}]++
		httpStats.sum += dur.Seconds()
		httpStats.total++
		httpStats.Unlock()
		if r.URL.Path != "/metrics" && r.URL.Path != "/healthz" && r.URL.Path != "/readyz" {
			addr, _ := s.clientAddr(r)
			slog.Info("http", "method", r.Method, "path", r.URL.Path,
				"status", status, "ms", dur.Milliseconds(), "ip", addr.String())
		}
	})
}
