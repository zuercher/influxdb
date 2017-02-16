package snapshotter_test

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"reflect"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/davecgh/go-spew/spew"
	"github.com/influxdata/influxdb/influxql"
	"github.com/influxdata/influxdb/services/meta"
	"github.com/influxdata/influxdb/services/snapshotter"
	"github.com/influxdata/influxdb/tcp"
	"github.com/influxdata/influxdb/tsdb"
)

var data = meta.Data{
	Databases: []meta.DatabaseInfo{
		{
			Name: "db0",
			DefaultRetentionPolicy: "autogen",
			RetentionPolicies: []meta.RetentionPolicyInfo{
				{
					Name:               "rp0",
					ReplicaN:           1,
					Duration:           24 * 7 * time.Hour,
					ShardGroupDuration: 24 * time.Hour,
					ShardGroups: []meta.ShardGroupInfo{
						{
							ID:        1,
							StartTime: time.Unix(0, 0).UTC(),
							EndTime:   time.Unix(0, 0).UTC().Add(24 * time.Hour),
							Shards: []meta.ShardInfo{
								{ID: 2},
							},
						},
					},
				},
				{
					Name:               "autogen",
					ReplicaN:           1,
					ShardGroupDuration: 24 * 7 * time.Hour,
					ShardGroups: []meta.ShardGroupInfo{
						{
							ID:        3,
							StartTime: time.Unix(0, 0).UTC(),
							EndTime:   time.Unix(0, 0).UTC().Add(24 * time.Hour),
							Shards: []meta.ShardInfo{
								{ID: 4},
							},
						},
					},
				},
			},
		},
	},
	Users: []meta.UserInfo{
		{
			Name:       "admin",
			Hash:       "abcxyz",
			Admin:      true,
			Privileges: map[string]influxql.Privilege{},
		},
	},
}

func TestSnapshotter_Open(t *testing.T) {
	s, l, err := NewTestService()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	if err := s.Open(); err != nil {
		t.Fatalf("unexpected open error: %s", err)
	}

	if err := s.Close(); err != nil {
		t.Fatalf("unexpected close error: %s", err)
	}
}

func TestSnapshotter_RequestShardBackup(t *testing.T) {
	s, l, err := NewTestService()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	var tsdb TSDBStore
	tsdb.BackupShardFn = func(id uint64, since time.Time, w io.Writer) error {
		if id != 5 {
			t.Errorf("unexpected shard id: got=%#v want=%#v", id, 5)
		}
		if got, want := since, time.Unix(0, 0).UTC(); !got.Equal(want) {
			t.Errorf("unexpected time since: got=%#v want=%#v", got, want)
		}
		// Write some nonsense data so we can check that it gets returned.
		w.Write([]byte(`{"status":"ok"}`))
		return nil
	}
	s.TSDBStore = &tsdb

	if err := s.Open(); err != nil {
		t.Fatalf("unexpected open error: %s", err)
	}
	defer s.Close()

	conn, err := net.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Errorf("unexpected error: %s", err)
		return
	}
	defer conn.Close()

	req := snapshotter.Request{
		Type:    snapshotter.RequestShardBackup,
		ShardID: 5,
		Since:   time.Unix(0, 0),
	}
	conn.Write([]byte{snapshotter.MuxHeader})
	enc := json.NewEncoder(conn)
	if err := enc.Encode(&req); err != nil {
		t.Errorf("unable to encode request: %s", err)
		return
	}

	// Read the result.
	out, err := ioutil.ReadAll(conn)
	if err != nil {
		t.Errorf("unexpected error reading shard backup: %s", err)
		return
	}

	if got, want := string(out), `{"status":"ok"}`; got != want {
		t.Errorf("unexpected shard data: got=%#v want=%#v", got, want)
		return
	}
}

func TestSnapshotter_RequestMetastoreBackup(t *testing.T) {
	s, l, err := NewTestService()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	s.MetaClient = &MetaClient{Data: data}
	if err := s.Open(); err != nil {
		t.Fatalf("unexpected open error: %s", err)
	}
	defer s.Close()

	conn, err := net.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Errorf("unexpected error: %s", err)
		return
	}
	defer conn.Close()

	c := snapshotter.NewClient(l.Addr().String())
	if got, err := c.MetastoreBackup(); err != nil {
		t.Errorf("unable to obtain metastore backup: %s", err)
		return
	} else if want := &data; !reflect.DeepEqual(got, want) {
		t.Errorf("unexpected data backup:\n\ngot=%s\nwant=%s", spew.Sdump(got), spew.Sdump(want))
		return
	}
}

func TestSnapshotter_RequestDatabaseInfo(t *testing.T) {
	s, l, err := NewTestService()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	var tsdbStore TSDBStore
	tsdbStore.ShardFn = func(id uint64) *tsdb.Shard {
		if id != 2 && id != 4 {
			t.Errorf("unexpected shard id: %d", id)
			return nil
		}
		return &tsdb.Shard{}
	}
	tsdbStore.ShardRelativePathFn = func(id uint64) (string, error) {
		if id == 2 {
			return "db0/rp0", nil
		} else if id == 4 {
			return "db0/autogen", nil
		}
		return "", fmt.Errorf("no such shard id: %d", id)
	}

	s.MetaClient = &MetaClient{Data: data}
	s.TSDBStore = &tsdbStore
	if err := s.Open(); err != nil {
		t.Fatalf("unexpected open error: %s", err)
	}
	defer s.Close()

	conn, err := net.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Errorf("unexpected error: %s", err)
		return
	}
	defer conn.Close()

	req := snapshotter.Request{
		Type:     snapshotter.RequestDatabaseInfo,
		Database: "db0",
	}
	conn.Write([]byte{snapshotter.MuxHeader})
	enc := json.NewEncoder(conn)
	if err := enc.Encode(&req); err != nil {
		t.Errorf("unable to encode request: %s", err)
		return
	}

	// Read the result.
	out, err := ioutil.ReadAll(conn)
	if err != nil {
		t.Errorf("unexpected error reading database info: %s", err)
		return
	}

	// Unmarshal the response.
	var resp snapshotter.Response
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Errorf("error unmarshaling response: %s", err)
		return
	}

	if got, want := resp.Paths, []string{"db0/rp0", "db0/autogen"}; !reflect.DeepEqual(got, want) {
		t.Errorf("unexpected paths: got=%#v want=%#v", got, want)
	}
}

func TestSnapshotter_RequestRetentionPolicyInfo(t *testing.T) {
	s, l, err := NewTestService()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	var tsdbStore TSDBStore
	tsdbStore.ShardFn = func(id uint64) *tsdb.Shard {
		if id != 2 {
			t.Errorf("unexpected shard id: %d", id)
			return nil
		}
		return &tsdb.Shard{}
	}
	tsdbStore.ShardRelativePathFn = func(id uint64) (string, error) {
		if id == 2 {
			return "db0/rp0", nil
		}
		return "", fmt.Errorf("no such shard id: %d", id)
	}

	s.MetaClient = &MetaClient{Data: data}
	s.TSDBStore = &tsdbStore
	if err := s.Open(); err != nil {
		t.Fatalf("unexpected open error: %s", err)
	}
	defer s.Close()

	conn, err := net.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Errorf("unexpected error: %s", err)
		return
	}
	defer conn.Close()

	req := snapshotter.Request{
		Type:            snapshotter.RequestRetentionPolicyInfo,
		Database:        "db0",
		RetentionPolicy: "rp0",
	}
	conn.Write([]byte{snapshotter.MuxHeader})
	enc := json.NewEncoder(conn)
	if err := enc.Encode(&req); err != nil {
		t.Errorf("unable to encode request: %s", err)
		return
	}

	// Read the result.
	out, err := ioutil.ReadAll(conn)
	if err != nil {
		t.Errorf("unexpected error reading database info: %s", err)
		return
	}

	// Unmarshal the response.
	var resp snapshotter.Response
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Errorf("error unmarshaling response: %s", err)
		return
	}

	if got, want := resp.Paths, []string{"db0/rp0"}; !reflect.DeepEqual(got, want) {
		t.Errorf("unexpected paths: got=%#v want=%#v", got, want)
	}
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

type TSDBStore struct {
	BackupShardFn       func(id uint64, since time.Time, w io.Writer) error
	ShardFn             func(id uint64) *tsdb.Shard
	ShardRelativePathFn func(id uint64) (string, error)
}

func (s *TSDBStore) BackupShard(id uint64, since time.Time, w io.Writer) error {
	return s.BackupShardFn(id, since, w)
}

func (s *TSDBStore) Shard(id uint64) *tsdb.Shard {
	return s.ShardFn(id)
}

func (s *TSDBStore) ShardRelativePath(id uint64) (string, error) {
	return s.ShardRelativePathFn(id)
}

type MetaClient struct {
	Data meta.Data
}

func (m *MetaClient) MarshalBinary() ([]byte, error) {
	return m.Data.MarshalBinary()
}

func (m *MetaClient) Database(name string) *meta.DatabaseInfo {
	for _, dbi := range m.Data.Databases {
		if dbi.Name == name {
			return &dbi
		}
	}
	return nil
}
