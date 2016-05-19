package tsdb

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/influxdata/influxdb/toml"
)

const (
	// DefaultEngine is the default engine for new shards
	DefaultEngine = "tsm1"

	// tsdb/engine/wal configuration options

	// Default settings for TSM

	// DefaultCacheMaxMemorySize is the maximum size a shard's cache can
	// reach before it starts rejecting writes.
	DefaultCacheMaxMemorySize = 500 * 1024 * 1024 // 500MB

	// DefaultCacheSnapshotMemorySize is the size at which the engine will
	// snapshot the cache and write it to a TSM file, freeing up memory
	DefaultCacheSnapshotMemorySize = 25 * 1024 * 1024 // 25MB

	// DefaultCacheSnapshotWriteColdDuration is the length of time at which
	// the engine will snapshot the cache and write it to a new TSM file if
	// the shard hasn't received writes or deletes
	DefaultCacheSnapshotWriteColdDuration = time.Duration(time.Hour)

	// DefaultCompactFullWriteColdDuration is the duration at which the engine
	// will compact all TSM files in a shard if it hasn't received a write or delete
	DefaultCompactFullWriteColdDuration = time.Duration(24 * time.Hour)

	// DefaultMaxPointsPerBlock is the maximum number of points in an encoded
	// block in a TSM file
	DefaultMaxPointsPerBlock = 1000
)

// Config holds the configuration for the tsbd package.
type Config struct {
	Dir    string `toml:"dir"`
	Engine string `toml:"engine"`

	// General WAL configuration options
	WALDir            string `toml:"wal-dir"`
	WALLoggingEnabled bool   `toml:"wal-logging-enabled"`

	// Query logging
	QueryLogEnabled bool `toml:"query-log-enabled"`

	// Compaction options for tsm1 (descriptions above with defaults)
	CacheMaxMemorySize             uint64        `toml:"cache-max-memory-size"`
	CacheSnapshotMemorySize        uint64        `toml:"cache-snapshot-memory-size"`
	CacheSnapshotWriteColdDuration toml.Duration `toml:"cache-snapshot-write-cold-duration"`
	CompactFullWriteColdDuration   toml.Duration `toml:"compact-full-write-cold-duration"`
	MaxPointsPerBlock              int           `toml:"max-points-per-block"`

	DataLoggingEnabled bool          `toml:"data-logging-enabled"`
	DiskThreshold      DiskThreshold `toml:"disk-threshold"`
}

// NewConfig returns the default configuration for tsdb.
func NewConfig() Config {
	return Config{
		Engine: DefaultEngine,

		WALLoggingEnabled: true,

		QueryLogEnabled: true,

		CacheMaxMemorySize:             DefaultCacheMaxMemorySize,
		CacheSnapshotMemorySize:        DefaultCacheSnapshotMemorySize,
		CacheSnapshotWriteColdDuration: toml.Duration(DefaultCacheSnapshotWriteColdDuration),
		CompactFullWriteColdDuration:   toml.Duration(DefaultCompactFullWriteColdDuration),

		DataLoggingEnabled: true,
	}
}

// Validate validates the configuration hold by c.
func (c *Config) Validate() error {
	if c.Dir == "" {
		return errors.New("Data.Dir must be specified")
	} else if c.WALDir == "" {
		return errors.New("Data.WALDir must be specified")
	}

	valid := false
	for _, e := range RegisteredEngines() {
		if e == c.Engine {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("unrecognized engine %s", c.Engine)
	}

	return nil
}

// DiskThreshold calculates the disk threshold using one of several different methods.
type DiskThreshold struct {
	Provider DiskThresholdProvider
}

func (d *DiskThreshold) Threshold(path string) (uint64, error) {
	if d.Provider != nil {
		return d.Provider.Threshold(path)
	}
	return 0, nil
}

var regexpDiskSize = regexp.MustCompile(`(?i)^(\d+)([kmgt]b?|%(FREE|USED)?)?`)

func (d *DiskThreshold) UnmarshalTOML(v interface{}) error {
	switch v := v.(type) {
	case int64:
		if v < 0 {
			return fmt.Errorf("invalid disk size: %d", v)
		}
		d.Provider = &StaticDiskThresholdProvider{Val: uint64(v)}
	case string:
		m := regexpDiskSize.FindStringSubmatch(v)
		if m == nil {
			return fmt.Errorf("invalid disk expression: %s", v)
		}

		n, err := strconv.ParseUint(m[1], 10, 64)
		if err != nil {
			return err
		}

		switch m[2] {
		case "k", "kb":
			d.Provider = &StaticDiskThresholdProvider{Val: 1024 * n}
		case "m", "mb":
			d.Provider = &StaticDiskThresholdProvider{Val: 1024 * 1024 * n}
		case "g", "gb":
			d.Provider = &StaticDiskThresholdProvider{Val: 1024 * 1024 * 1024 * n}
		case "t", "tb":
			d.Provider = &StaticDiskThresholdProvider{Val: 1024 * 1024 * 1024 * n}
		case "%", "%FREE", "%USED":
			if n > 100 {
				return fmt.Errorf("invalid percentage: %d", n)
			} else if m[2] == "%USED" {
				n = 100 - n
			}
			d.Provider = &PercentageDiskThresholdProvider{Val: int(n)}
		default:
			d.Provider = &StaticDiskThresholdProvider{Val: n}
		}
	default:
		return fmt.Errorf("invalid disk threshold value: %s (%T)", v, v)
	}
	return nil
}

type DiskThresholdProvider interface {
	// Threshold calculates the byte size threshold for the passed in path.
	// Static providers may not require the path information, but percentages
	// need to know the path so they can calculate the maximum size and take a
	// percentage of that size.
	Threshold(path string) (uint64, error)
}

type StaticDiskThresholdProvider struct {
	Val uint64
}

func (d *StaticDiskThresholdProvider) Threshold(path string) (uint64, error) {
	return d.Val, nil
}

type PercentageDiskThresholdProvider struct {
	Val int
}

func (d *PercentageDiskThresholdProvider) Threshold(path string) (uint64, error) {
	stat, err := diskstat(path)
	if err != nil {
		return 0, err
	}
	return uint64(d.Val) * stat.Total / 100, nil
}
