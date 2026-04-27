package dedup

import (
	"fmt"
	"sync"

	"github.com/ChrisEineke/go-events"
	"github.com/chigopher/pathlib"
)

const (
	EvtFileHashAdded = "db.file_hash_added"
)

type InMemDatabase struct {
	fileToHash  map[pathlib.Path]string
	hashToFiles map[string][]*pathlib.Path
	sync.RWMutex
	OnFileHashAdded events.E
}

func NewInMemDatabase() *InMemDatabase {
	db := &InMemDatabase{
		fileToHash:      map[pathlib.Path]string{},
		hashToFiles:     map[string][]*pathlib.Path{},
		RWMutex:         sync.RWMutex{},
		OnFileHashAdded: events.E{N: EvtFileHashAdded},
	}
	_ = Database(db)
	return db
}

func (db *InMemDatabase) Open() error {
	return nil
}

func (db *InMemDatabase) Close() error {
	return nil
}

func (db *InMemDatabase) AddFileHash(filepath *pathlib.Path, hash string) error {
	db.Lock()
	defer db.Unlock()

	if curHash, ok := db.fileToHash[*filepath]; ok {
		return fmt.Errorf("file %s already has hash %s", filepath.String(), curHash)
	}
	db.fileToHash[*filepath] = hash
	db.hashToFiles[hash] = append(db.hashToFiles[hash], filepath)
	db.OnFileHashAdded.Fire(filepath, hash)
	return nil
}

func (db *InMemDatabase) FindDuplicateHashes() ([]*DuplicateHash, error) {
	db.RLock()
	defer db.RUnlock()

	result := []*DuplicateHash{}
	for hash, files := range db.hashToFiles {
		if len(files) < 2 {
			continue
		}
		result = append(result, &DuplicateHash{Hash: hash, Files: files})
	}
	return result, nil
}
