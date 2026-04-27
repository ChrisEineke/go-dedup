package dedup

import (
	"fmt"
	"time"

	"github.com/chigopher/pathlib"
)

const MaxDuration time.Duration = 1<<63 - 1

type Database interface {
	Open() error
	Close() error

	AddFileHash(filepath *pathlib.Path, hash string) error
	FindDuplicateHashes() ([]*DuplicateHash, error)
}

type DuplicateHash struct {
	Hash  string
	Files []*pathlib.Path
}

func (dh *DuplicateHash) String() string {
	return fmt.Sprintf("DuplicateHash{%v (%v)}", dh.Hash, dh.Files)
}
