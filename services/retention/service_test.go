package retention_test

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/influxdata/influxdb/services/meta"
	"github.com/influxdata/influxdb/services/retention"
	"github.com/influxdata/influxdb/toml"
)

func TestRetention(t *testing.T) {
	now := time.Now()
	// Account for any time difference that could cause some of the logic in
	// this test to fail due to a race condition. If we are at the very end of
	// the hour, we can choose a time interval based on one "now" time and then
	// run the retention service in the next hour. If we're in one of those
	// situations, wait 100 milliseconds until we're in the next hour.
	if got, want := now.Add(100*time.Millisecond).Truncate(time.Hour), now.Truncate(time.Hour); !got.Equal(want) {
		time.Sleep(100 * time.Millisecond)
	}

	data := []meta.DatabaseInfo{
		{
			Name: "db0",
			DefaultRetentionPolicy: "rp0",
			RetentionPolicies: []meta.RetentionPolicyInfo{
				{
					Name:               "rp0",
					ReplicaN:           1,
					Duration:           time.Hour,
					ShardGroupDuration: time.Hour,
					ShardGroups: []meta.ShardGroupInfo{
						{
							ID:        1,
							StartTime: now.Truncate(time.Hour).Add(-2 * time.Hour),
							EndTime:   now.Truncate(time.Hour).Add(-1 * time.Hour),
							Shards: []meta.ShardInfo{
								{ID: 2},
								{ID: 3},
							},
						},
						{
							ID:        4,
							StartTime: now.Truncate(time.Hour).Add(-1 * time.Hour),
							EndTime:   now.Truncate(time.Hour),
							Shards: []meta.ShardInfo{
								{ID: 5},
								{ID: 6},
							},
						},
						{
							ID:        7,
							StartTime: now.Truncate(time.Hour),
							EndTime:   now.Truncate(time.Hour).Add(time.Hour),
							Shards: []meta.ShardInfo{
								{ID: 8},
								{ID: 9},
							},
						},
					},
				},
			},
		},
	}
	var mc MetaClient
	mc.DatabasesFn = func() []meta.DatabaseInfo {
		return data
	}

	done := make(chan struct{})
	deletedShardGroups := make(map[string]struct{})
	mc.DeleteShardGroupFn = func(database, policy string, id uint64) error {
		for _, dbi := range data {
			if dbi.Name == database {
				for _, rpi := range dbi.RetentionPolicies {
					if rpi.Name == policy {
						for i, sg := range rpi.ShardGroups {
							if sg.ID == id {
								rpi.ShardGroups[i].DeletedAt = time.Now().UTC()
							}
						}
					}
				}
			}
		}

		deletedShardGroups[fmt.Sprintf("%s.%s.%d", database, policy, id)] = struct{}{}
		if got, want := deletedShardGroups, map[string]struct{}{
			"db0.rp0.1": struct{}{},
		}; reflect.DeepEqual(got, want) {
			close(done)
		} else if len(got) > 1 {
			t.Errorf("deleted too many shard groups")
		}
		return nil
	}

	pruned := false
	closing := make(chan struct{})
	mc.PruneShardGroupsFn = func() error {
		select {
		case <-done:
			if !pruned {
				close(closing)
				pruned = true
			}
		default:
		}
		return nil
	}

	deletedShards := make(map[uint64]struct{})
	var tsdbStore TSDBStore
	tsdbStore.ShardIDsFn = func() []uint64 {
		return []uint64{2, 3, 5, 6}
	}
	tsdbStore.DeleteShardFn = func(shardID uint64) error {
		deletedShards[shardID] = struct{}{}
		return nil
	}

	s := NewTestService()
	s.MetaClient = &mc
	s.TSDBStore = &tsdbStore
	if err := s.Open(); err != nil {
		t.Fatalf("unexpected open error: %s", err)
	}
	defer func() {
		if err := s.Close(); err != nil {
			t.Fatalf("unexpected close error: %s", err)
		}
	}()

	timer := time.NewTimer(100 * time.Millisecond)
	select {
	case <-done:
		timer.Stop()
	case <-timer.C:
		t.Errorf("timeout waiting for shard groups to be deleted")
		return
	}

	timer = time.NewTimer(100 * time.Millisecond)
	select {
	case <-closing:
		timer.Stop()
	case <-timer.C:
		t.Errorf("timeout waiting for shards to be deleted")
		return
	}

	if got, want := deletedShards, map[uint64]struct{}{
		2: struct{}{},
		3: struct{}{},
	}; !reflect.DeepEqual(got, want) {
		t.Errorf("unexpected deleted shards: got=%#v want=%#v", got, want)
	}
}

func NewTestService() *retention.Service {
	config := retention.NewConfig()
	config.CheckInterval = toml.Duration(10 * time.Millisecond)

	s := retention.NewService(config)
	s.WithLogger(zap.New(zap.NewTextEncoder()))
	return s
}

type MetaClient struct {
	DatabasesFn        func() []meta.DatabaseInfo
	DeleteShardGroupFn func(database, policy string, id uint64) error
	PruneShardGroupsFn func() error
}

func (m *MetaClient) Databases() []meta.DatabaseInfo {
	return m.DatabasesFn()
}

func (m *MetaClient) DeleteShardGroup(database, policy string, id uint64) error {
	return m.DeleteShardGroupFn(database, policy, id)
}

func (m *MetaClient) PruneShardGroups() error {
	return m.PruneShardGroupsFn()
}

type TSDBStore struct {
	ShardIDsFn    func() []uint64
	DeleteShardFn func(shardID uint64) error
}

func (s *TSDBStore) ShardIDs() []uint64 {
	return s.ShardIDsFn()
}

func (s *TSDBStore) DeleteShard(shardID uint64) error {
	return s.DeleteShardFn(shardID)
}
