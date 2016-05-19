// +build !windows
package tsdb

import "syscall"

func diskstat(path string) (diskStat, error) {
	var fs syscall.Statfs_t
	if err := syscall.Statfs(path, &fs); err != nil {
		return diskStat{}, err
	}

	return diskStat{
		Total: fs.Blocks * uint64(fs.Bsize),
		Free:  fs.Bfree * uint64(fs.Bsize),
	}, nil
}
