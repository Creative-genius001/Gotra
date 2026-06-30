// Package analytics implements the ClickHouse-backed analytics pipeline
// (Backend Bible — Analytics Pipeline). It is optional: when CLICKHOUSE_URL is
// unset or unreachable, the Store degrades to a no-op so the platform runs on
// Postgres + Redis alone. The gateway writes request events; the API queries
// aggregates.
package analytics

import (
	"context"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/uuid"
)

const schema = `
CREATE TABLE IF NOT EXISTS request_events (
    project_id  UUID,
    tunnel_id   UUID,
    method      LowCardinality(String),
    path        String,
    status      UInt16,
    duration_ms UInt32,
    received_at DateTime
) ENGINE = MergeTree ORDER BY (project_id, received_at)`

// Event is a single captured request, recorded for analytics.
type Event struct {
	ProjectID  uuid.UUID
	TunnelID   uuid.UUID
	Method     string
	Path       string
	Status     int
	DurationMs int
	ReceivedAt time.Time
}

// Summary aggregates traffic for a project over a window.
type Summary struct {
	Enabled    bool          `json:"enabled"`
	Requests   uint64        `json:"requests"`
	Errors     uint64        `json:"errors"`
	ErrorRate  float64       `json:"error_rate"`
	AvgLatency float64       `json:"avg_latency_ms"`
	P95Latency float64       `json:"p95_latency_ms"`
	Series     []SeriesPoint `json:"series"`
}

// SeriesPoint is one bucket of the traffic time series.
type SeriesPoint struct {
	Bucket   time.Time `json:"bucket"`
	Requests uint64    `json:"requests"`
	Errors   uint64    `json:"errors"`
}

// Store is the analytics writer + reader. The zero value (disabled) is usable.
type Store struct {
	conn driver.Conn
	log  *slog.Logger

	mu  sync.Mutex
	buf []Event
	ch  chan Event
	done chan struct{}
}

// Open connects to ClickHouse and ensures the schema. On any failure (including
// an empty URL) it returns a disabled Store rather than an error — analytics is
// best-effort and must never block startup.
func Open(ctx context.Context, dsn string, log *slog.Logger) *Store {
	if dsn == "" {
		log.Info("analytics disabled (CLICKHOUSE_URL not set)")
		return &Store{log: log}
	}

	opts, err := clickhouse.ParseDSN(dsn)
	if err != nil {
		log.Warn("analytics disabled: bad CLICKHOUSE_URL", "error", err)
		return &Store{log: log}
	}
	conn, err := clickhouse.Open(opts)
	if err != nil {
		log.Warn("analytics disabled: connect failed", "error", err)
		return &Store{log: log}
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := conn.Ping(pingCtx); err != nil {
		log.Warn("analytics disabled: ping failed", "error", err)
		return &Store{log: log}
	}
	if err := conn.Exec(ctx, schema); err != nil {
		log.Warn("analytics disabled: schema failed", "error", err)
		return &Store{log: log}
	}

	s := &Store{
		conn: conn,
		log:  log,
		ch:   make(chan Event, 1024),
		done: make(chan struct{}),
	}
	go s.flushLoop()
	log.Info("analytics enabled (clickhouse)")
	return s
}

// Enabled reports whether analytics is connected.
func (s *Store) Enabled() bool { return s.conn != nil }

// Record buffers an event for asynchronous insertion. No-op when disabled.
func (s *Store) Record(e Event) {
	if s.conn == nil {
		return
	}
	select {
	case s.ch <- e:
	default: // drop on overload rather than block the request path
	}
}

func (s *Store) flushLoop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.done:
			s.flush()
			return
		case e := <-s.ch:
			s.mu.Lock()
			s.buf = append(s.buf, e)
			n := len(s.buf)
			s.mu.Unlock()
			if n >= 500 {
				s.flush()
			}
		case <-ticker.C:
			s.flush()
		}
	}
}

func (s *Store) flush() {
	s.mu.Lock()
	if len(s.buf) == 0 {
		s.mu.Unlock()
		return
	}
	batch := s.buf
	s.buf = nil
	s.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	b, err := s.conn.PrepareBatch(ctx, "INSERT INTO request_events")
	if err != nil {
		s.log.Warn("analytics batch prepare failed", "error", err)
		return
	}
	for _, e := range batch {
		if err := b.Append(e.ProjectID, e.TunnelID, e.Method, e.Path, uint16(e.Status), uint32(e.DurationMs), e.ReceivedAt); err != nil {
			s.log.Warn("analytics append failed", "error", err)
		}
	}
	if err := b.Send(); err != nil {
		s.log.Warn("analytics flush failed", "error", err)
	}
}

// QuerySummary returns aggregated traffic for a project since the given time.
func (s *Store) QuerySummary(ctx context.Context, projectID uuid.UUID, since time.Time) (Summary, error) {
	if s.conn == nil {
		return Summary{Enabled: false, Series: []SeriesPoint{}}, nil
	}

	var (
		requests, errors uint64
		avg, p95         float64
	)
	row := s.conn.QueryRow(ctx,
		`SELECT count(), countIf(status >= 400), avg(duration_ms), quantile(0.95)(duration_ms)
		 FROM request_events WHERE project_id = ? AND received_at >= ?`, projectID, since)
	if err := row.Scan(&requests, &errors, &avg, &p95); err != nil {
		return Summary{}, err
	}
	// avg()/quantile() return NaN over an empty window, which is not JSON-encodable.
	if math.IsNaN(avg) || math.IsInf(avg, 0) {
		avg = 0
	}
	if math.IsNaN(p95) || math.IsInf(p95, 0) {
		p95 = 0
	}

	series := []SeriesPoint{}
	rows, err := s.conn.Query(ctx,
		`SELECT toStartOfInterval(received_at, INTERVAL 1 HOUR) AS bucket, count(), countIf(status >= 400)
		 FROM request_events WHERE project_id = ? AND received_at >= ?
		 GROUP BY bucket ORDER BY bucket`, projectID, since)
	if err != nil {
		return Summary{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var p SeriesPoint
		if err := rows.Scan(&p.Bucket, &p.Requests, &p.Errors); err != nil {
			return Summary{}, err
		}
		series = append(series, p)
	}

	rate := 0.0
	if requests > 0 {
		rate = float64(errors) / float64(requests)
	}
	return Summary{
		Enabled: true, Requests: requests, Errors: errors, ErrorRate: rate,
		AvgLatency: avg, P95Latency: p95, Series: series,
	}, nil
}

// Close stops the writer and closes the connection.
func (s *Store) Close() {
	if s.conn == nil {
		return
	}
	close(s.done)
	_ = s.conn.Close()
}
