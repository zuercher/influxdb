package tsdb

import "errors"

func diskstat(path string) (diskStat, error) {
	return diskStat{}, errors.New("diskstat not implemented on windows")
}
