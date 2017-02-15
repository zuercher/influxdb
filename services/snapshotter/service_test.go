package snapshotter_test

import (
	"net"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/influxdata/influxdb/services/snapshotter"
	"github.com/influxdata/influxdb/tcp"
)

func TestSnapshotter_Open(t *testing.T) {
	s, l, err := NewTestService()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	if err := s.Open(); err != nil {
		t.Fatalf("unexpected open error: %s", err)
	}
	time.Sleep(time.Second)

	if err := s.Close(); err != nil {
		t.Fatalf("unexpected close error: %s", err)
	}
}

func TestSnapshotter_RequestShardBackup(t *testing.T) {
	/*
		s, err := NewTestService()
		if err != nil {
			t.Fatal(err)
		}

		if err := s.Open(); err != nil {
			t.Fatalf("unexpected open error: %s", err)
		}
		defer s.Close()
	*/
}

func NewTestService() (*snapshotter.Service, net.Listener, error) {
	logger := zap.New(zap.NewTextEncoder())
	s := snapshotter.NewService()
	s.WithLogger(logger)

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, nil, err
	}

	// The snapshotter needs to be used with a tcp.Mux listener.
	mux := tcp.NewMux()
	go mux.Serve(l)

	s.Listener = mux.Listen(snapshotter.MuxHeader)
	return s, l, nil
}
