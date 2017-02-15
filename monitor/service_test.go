package monitor_test

import (
	"testing"
	"time"

	"github.com/influxdata/influxdb/models"
	"github.com/influxdata/influxdb/monitor"
	"github.com/influxdata/influxdb/services/meta"
	"github.com/influxdata/influxdb/toml"
)

func TestMonitor_Open(t *testing.T) {
	s := monitor.New(nil, monitor.Config{})
	if err := s.Open(); err != nil {
		t.Fatalf("unexpected open error: %s", err)
	}

	// Verify that opening twice is fine.
	if err := s.Open(); err != nil {
		s.Close()
		t.Fatalf("unexpected error on second open: %s", err)
	}

	if err := s.Close(); err != nil {
		t.Fatalf("unexpected close error: %s", err)
	}

	// Verify that closing twice is fine.
	if err := s.Close(); err != nil {
		t.Fatalf("unexpected error on second close: %s", err)
	}
}

func TestMonitor_StoreStatistics(t *testing.T) {
	reporter := ReporterFunc(func(tags map[string]string) []models.Statistic {
		return []models.Statistic{}
	})

	done := make(chan struct{})
	defer close(done)
	ch := make(chan models.Points)

	var mc MetaClient
	mc.CreateDatabaseWithRetentionPolicyFn = func(name string, spec *meta.RetentionPolicySpec) (*meta.DatabaseInfo, error) {
		if got, want := name, monitor.DefaultStoreDatabase; got != want {
			t.Errorf("unexpected database: got=%q want=%q", got, want)
		}
		if got, want := spec.Name, monitor.MonitorRetentionPolicy; got != want {
			t.Errorf("unexpected retention policy: got=%q want=%q", got, want)
		}
		if spec.Duration != nil {
			if got, want := *spec.Duration, monitor.MonitorRetentionPolicyDuration; got != want {
				t.Errorf("unexpected duration: got=%q want=%q", got, want)
			}
		} else {
			t.Error("expected duration in retention policy spec")
		}
		if spec.ReplicaN != nil {
			if got, want := *spec.ReplicaN, monitor.MonitorRetentionPolicyReplicaN; got != want {
				t.Errorf("unexpected replica number: got=%q want=%q", got, want)
			}
		} else {
			t.Error("expected replica number in retention policy spec")
		}
		return &meta.DatabaseInfo{Name: name}, nil
	}

	var pw PointsWriter
	pw.WritePointsFn = func(database, policy string, points models.Points) error {
		// Verify that we are attempting to write to the correct database.
		if got, want := database, monitor.DefaultStoreDatabase; got != want {
			t.Errorf("unexpected database: got=%q want=%q", got, want)
		}
		if got, want := policy, monitor.MonitorRetentionPolicy; got != want {
			t.Errorf("unexpected retention policy: got=%q want=%q", got, want)
		}

		// Attempt to write the points to the main goroutine.
		select {
		case <-done:
		case ch <- points:
		}
		return nil
	}

	config := monitor.NewConfig()
	config.StoreInterval = toml.Duration(10 * time.Millisecond)
	s := monitor.New(reporter, config)
	s.MetaClient = &mc
	s.PointsWriter = &pw

	if err := s.Open(); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	defer s.Close()

	timer := time.NewTimer(100 * time.Millisecond)
	select {
	case points := <-ch:
		timer.Stop()
		if got, want := points.Len(), 1; got != want {
			t.Errorf("unexpected number of points: got=%v want=%v", got, want)
		} else {
			p := points[0]
			if got, want := p.Name(), "runtime"; got != want {
				t.Errorf("unexpected point name: got=%q want=%q", got, want)
			}
			// There should be a hostname.
			if got := p.Tags().GetString("hostname"); len(got) == 0 {
				t.Errorf("expected hostname tag")
			}
			// This should write on an exact interval of 10 milliseconds.
			if got, want := p.Time(), p.Time().Truncate(10*time.Millisecond); got != want {
				t.Errorf("unexpected time: got=%q want=%q", got, want)
			}
		}
	case <-timer.C:
		t.Errorf("timeout while waiting for statistics to be written")
	}
}

type ReporterFunc func(tags map[string]string) []models.Statistic

func (f ReporterFunc) Statistics(tags map[string]string) []models.Statistic {
	return f(tags)
}

type PointsWriter struct {
	WritePointsFn func(database, policy string, points models.Points) error
}

func (pw *PointsWriter) WritePoints(database, policy string, points models.Points) error {
	return pw.WritePointsFn(database, policy, points)
}

type MetaClient struct {
	CreateDatabaseWithRetentionPolicyFn func(name string, spec *meta.RetentionPolicySpec) (*meta.DatabaseInfo, error)
	DatabaseFn                          func(name string) *meta.DatabaseInfo
}

func (m *MetaClient) CreateDatabaseWithRetentionPolicy(name string, spec *meta.RetentionPolicySpec) (*meta.DatabaseInfo, error) {
	return m.CreateDatabaseWithRetentionPolicyFn(name, spec)
}

func (m *MetaClient) Database(name string) *meta.DatabaseInfo {
	if m.DatabaseFn != nil {
		return m.DatabaseFn(name)
	}
	return nil
}
