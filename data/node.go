package data

import (
	"sync"

	"github.com/influxdb/influxdb/tsdb"
)

func NewDataNode(id uint64) *Node {
	return &Node{
		id:          id,
		localShards: make(map[uint64]*tsdb.Shard),
	}
}

type Node struct {
	id uint64

	mu          sync.RWMutex
	localShards map[uint64]*tsdb.Shard

	ClusterWriter interface {
		Write(shardID, nodeID uint64, points tsdb.Point) error
	}
}

func (n *Node) Open() error {
	// Open shards
	// Start AE for Node
	return nil
}

func (n *Node) Write(shardID, nodeID uint64, points tsdb.Point) error {
	if n.id != nodeID {
		n.ClusterWriter.Write(shardID, nodeID, points)
	}

	// TODO: implement local shard write
	return nil
}

func (n *Node) Close() error { return nil }
func (n *Node) Init() error  { return nil }

func (n *Node) WriteShard(shardID uint64, points []tsdb.Point) (int, error) {
	return 0, nil
}
