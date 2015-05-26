package data_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/influxdb/influxdb/data"
	"github.com/influxdb/influxdb/meta"
	"github.com/influxdb/influxdb/test"
	"github.com/influxdb/influxdb/tsdb"
)

type fakeShardWriter struct {
	ShardWriteFn func(shardID, nodeID uint64, points []tsdb.Point) error
}

func (f *fakeShardWriter) Write(shardID, nodeID uint64, points []tsdb.Point) error {
	return f.ShardWriteFn(shardID, nodeID, points)
}

func newTestMetaStore() *test.MetaStore {
	ms := &test.MetaStore{}
	rp := test.NewRetentionPolicy("myp", time.Hour, 3)
	test.AttachShardGroupInfo(rp, []uint64{1, 2, 3})
	test.AttachShardGroupInfo(rp, []uint64{1, 2, 3})

	ms.RetentionPolicyFn = func(db, retentionPolicy string) (*meta.RetentionPolicyInfo, error) {
		return rp, nil
	}

	ms.CreateShardGroupIfNotExistsFn = func(database, policy string, timestamp time.Time) (*meta.ShardGroupInfo, error) {
		for i, sg := range rp.ShardGroups {
			if timestamp.Equal(sg.StartTime) || timestamp.After(sg.StartTime) && timestamp.Before(sg.EndTime) {
				return &rp.ShardGroups[i], nil
			}
		}
		panic("should not get here")
	}
	return ms
}

// TestCoordinatorEnsureShardMappingOne tests that a single point maps to
// a single shard
func TestCoordinatorEnsureShardMappingOne(t *testing.T) {
	ms := test.MetaStore{}
	rp := test.NewRetentionPolicy("myp", time.Hour, 3)

	ms.RetentionPolicyFn = func(db, retentionPolicy string) (*meta.RetentionPolicyInfo, error) {
		return rp, nil
	}

	ms.CreateShardGroupIfNotExistsFn = func(database, policy string, timestamp time.Time) (*meta.ShardGroupInfo, error) {
		return &rp.ShardGroups[0], nil
	}

	c := data.Coordinator{MetaStore: ms}
	pr := &data.WritePointsRequest{
		Database:         "mydb",
		RetentionPolicy:  "myrp",
		ConsistencyLevel: data.ConsistencyLevelOne,
	}
	pr.AddPoint("cpu", 1.0, time.Now(), nil)

	var (
		shardMappings *data.ShardMapping
		err           error
	)
	if shardMappings, err = c.MapShards(pr); err != nil {
		t.Fatalf("unexpected an error: %v", err)
	}

	if exp := 1; len(shardMappings.Points) != exp {
		t.Errorf("MapShards() len mismatch. got %v, exp %v", len(shardMappings.Points), exp)
	}
}

// TestCoordinatorEnsureShardMappingMultiple tests that MapShards maps multiple points
// across shard group boundaries to multiple shards
func TestCoordinatorEnsureShardMappingMultiple(t *testing.T) {
	ms := test.MetaStore{}
	rp := test.NewRetentionPolicy("myp", time.Hour, 3)
	test.AttachShardGroupInfo(rp, []uint64{1, 2, 3})
	test.AttachShardGroupInfo(rp, []uint64{1, 2, 3})

	ms.RetentionPolicyFn = func(db, retentionPolicy string) (*meta.RetentionPolicyInfo, error) {
		return rp, nil
	}

	ms.CreateShardGroupIfNotExistsFn = func(database, policy string, timestamp time.Time) (*meta.ShardGroupInfo, error) {
		for i, sg := range rp.ShardGroups {
			if timestamp.Equal(sg.StartTime) || timestamp.After(sg.StartTime) && timestamp.Before(sg.EndTime) {
				return &rp.ShardGroups[i], nil
			}
		}
		panic("should not get here")
	}

	c := data.Coordinator{MetaStore: ms}
	pr := &data.WritePointsRequest{
		Database:         "mydb",
		RetentionPolicy:  "myrp",
		ConsistencyLevel: data.ConsistencyLevelOne,
	}

	// Three points that range over the shardGroup duration (1h) and should map to two
	// distinct shards
	pr.AddPoint("cpu", 1.0, time.Unix(0, 0), nil)
	pr.AddPoint("cpu", 2.0, time.Unix(0, 0).Add(time.Hour), nil)
	pr.AddPoint("cpu", 3.0, time.Unix(0, 0).Add(time.Hour+time.Second), nil)

	var (
		shardMappings *data.ShardMapping
		err           error
	)
	if shardMappings, err = c.MapShards(pr); err != nil {
		t.Fatalf("unexpected an error: %v", err)
	}

	if exp := 2; len(shardMappings.Points) != exp {
		t.Errorf("MapShards() len mismatch. got %v, exp %v", len(shardMappings.Points), exp)
	}

	for _, points := range shardMappings.Points {
		// First shard shoud have 1 point w/ first point added
		if len(points) == 1 && points[0].Time() != pr.Points[0].Time() {
			t.Fatalf("MapShards() value mismatch. got %v, exp %v", points[0].Time(), pr.Points[0].Time())
		}

		// Second shard shoud have the last two points added
		if len(points) == 2 && points[0].Time() != pr.Points[1].Time() {
			t.Fatalf("MapShards() value mismatch. got %v, exp %v", points[0].Time(), pr.Points[1].Time())
		}

		if len(points) == 2 && points[1].Time() != pr.Points[2].Time() {
			t.Fatalf("MapShards() value mismatch. got %v, exp %v", points[1].Time(), pr.Points[2].Time())
		}
	}
}

func TestCoordinatorWrite(t *testing.T) {

	tests := []struct {
		name        string
		consistency data.ConsistencyLevel
		wrote       int
		err         error
		expErr      error
	}{
		// Consistency one
		{
			name:        "write one success",
			consistency: data.ConsistencyLevelOne,
			err:         fmt.Errorf("a failure"),
			wrote:       1,
			expErr:      nil,
		},
		{
			name:        "write one fail",
			consistency: data.ConsistencyLevelOne,
			wrote:       0,
			err:         fmt.Errorf("a failure"),
			expErr:      fmt.Errorf("a failure"),
		},

		// Consistency any
		{
			name:        "write any success",
			consistency: data.ConsistencyLevelAny,
			wrote:       1,
			err:         fmt.Errorf("a failure"),
			expErr:      nil,
		},
		{
			name:        "write any failure",
			consistency: data.ConsistencyLevelAny,
			wrote:       0,
			err:         fmt.Errorf("a failure"),
			expErr:      fmt.Errorf("a failure"),
		},

		// Consistency all
		{
			name:        "write all success",
			consistency: data.ConsistencyLevelAll,
			wrote:       3,
			expErr:      nil,
		},
		{
			name:        "write all, 2/3",
			consistency: data.ConsistencyLevelAll,
			wrote:       2,
			err:         fmt.Errorf("a failure"),
			expErr:      fmt.Errorf("a failure"),
		},
		{
			name:        "write all, 1/3 (failure)",
			consistency: data.ConsistencyLevelAll,
			wrote:       1,
			err:         fmt.Errorf("a failure"),
			expErr:      fmt.Errorf("a failure"),
		},

		// Consistency quorum
		{
			name:        "write quorum, 1/3 failure",
			consistency: data.ConsistencyLevelQuorum,
			wrote:       1,
			err:         fmt.Errorf("a failure"),
			expErr:      fmt.Errorf("a failure"),
		},
		{
			name:        "write quorum, 2/3 success",
			consistency: data.ConsistencyLevelQuorum,
			wrote:       2,
			err:         fmt.Errorf("a failure"),
			expErr:      nil,
		},
		{
			name:        "write quorum, 3/3 success",
			consistency: data.ConsistencyLevelQuorum,
			wrote:       3,
			expErr:      nil,
		},

		// Error write failed
		{
			name:        "no writes succeed",
			consistency: data.ConsistencyLevelOne,
			dnWrote:     0,
			expErr:      data.ErrWriteFailed,
		},
	}

	for _, test := range tests {

		// copy to prevent data race
		theTest := test
		sm := data.NewShardMapping()
		sm.MapPoint(
			&meta.ShardInfo{ID: uint64(1), OwnerIDs: []uint64{uint64(1), uint64(2), uint64(3)}},
			tsdb.NewPoint(
				"cpu",
				nil,
				map[string]interface{}{"value": 0.0},
				time.Unix(0, 0),
			))

		// Local data.Node ShardWriter
		// lock on the write increment since these functions get called in parallel
		var mu sync.Mutex
		wrote := 0
		dn := &fakeShardWriter{
			ShardWriteFn: func(shardID, nodeID uint64, points []tsdb.Point) error {
				mu.Lock()
				defer mu.Unlock()
				if wrote == theTest.dnWrote {
					return theTest.dnErr
				}
				wrote += 1
				return nil
			},
		}

		ms := newTestMetaStore()
		c := data.Coordinator{
			MetaStore: ms,
			Cluster:   dn,
		}

		pr := &data.WritePointsRequest{
			Database:         "mydb",
			RetentionPolicy:  "myrp",
			ConsistencyLevel: test.consistency,
		}

		// Three points that range over the shardGroup duration (1h) and should map to two
		// distinct shards
		pr.AddPoint("cpu", 1.0, time.Unix(0, 0), nil)
		pr.AddPoint("cpu", 2.0, time.Unix(0, 0).Add(time.Hour), nil)
		pr.AddPoint("cpu", 3.0, time.Unix(0, 0).Add(time.Hour+time.Second), nil)

		if err := c.Write(pr); err != test.expErr {
			t.Errorf("Coordinator.Write(): '%s' failed: got %v, exp %v", test.name, err, test.expErr)
		}
	}
}
