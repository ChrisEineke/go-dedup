package dedup

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"log/slog"
	"os"
	"time"

	events "github.com/ChrisEineke/go-events"
	"github.com/chigopher/pathlib"
)

const (
	EvtFileHashingStarted  = "hasher.hashing_started"
	EvtFileHashed          = "hasher.file_hashed"
	EvtFileHashingFinished = "hasher.hashing_finished"

	EvtDeduplicationStarted  = "deduplicator.deduplication_started"
	EvtDeduplicated          = "deduplicator.deduplicated"
	EvtDeduplicationFinished = "deduplicator.deduplication_finished"
)

type Deduplicator struct {
	canonicalDir            *pathlib.Path
	hash                    hash.Hash
	dryRun                  bool
	OnFileHashingStarted    events.E
	OnFileHashingFinished   events.E
	OnFileHashed            events.E
	OnDeduplicationStarted  events.E
	OnDeduplicationFinished events.E
	OnDeduplicated          events.E
}

func NewDeduplicator(canonicalDir *pathlib.Path, dryRun bool) *Deduplicator {
	deduplicator := &Deduplicator{
		canonicalDir:            canonicalDir,
		hash:                    sha256.New(),
		dryRun:                  dryRun,
		OnFileHashingStarted:    events.E{N: EvtFileHashingStarted},
		OnFileHashingFinished:   events.E{N: EvtFileHashingFinished},
		OnFileHashed:            events.E{N: EvtFileHashed},
		OnDeduplicationStarted:  events.E{N: EvtDeduplicationStarted},
		OnDeduplicationFinished: events.E{N: EvtDeduplicationFinished},
		OnDeduplicated:          events.E{N: EvtDeduplicated},
	}
	return deduplicator
}

func (d *Deduplicator) HashFiles(dirs []*pathlib.Path) error {
	d.OnFileHashingStarted.Fire()
	defer d.OnFileHashingFinished.Fire()

	if len(dirs) == 0 {
		return nil
	}
	for _, dir := range dirs {
		resolvedDir, err := dir.ResolveAll()
		if err != nil {
			return err
		}
		if relativeDir, err := resolvedDir.RelativeTo(d.canonicalDir); err == nil && len(relativeDir.Parts()) == 0 {
			// If pathlib failed to make the two paths relative to each other, then they're definitely not the same
			// directory.
			slog.Debug("skipping canonical directory", "dir", dir.String())
			continue
		}
		err = d.walkDirectory(dir)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *Deduplicator) walkDirectory(dir *pathlib.Path) error {
	if dir == nil {
		return nil
	}
	walk, err := pathlib.NewWalk(dir, pathlib.WalkSortChildren(true))
	if err != nil {
		return err
	}
	return walk.Walk(d.walkFunc)
}

func (d *Deduplicator) walkFunc(path *pathlib.Path, info os.FileInfo, err error) error {
	if !info.Mode().IsRegular() {
		slog.Debug("Skipped non-regular file for hashing", "file", path)
		return nil
	}
	fileHash, err := d.hashFile(path)
	if err != nil {
		return err
	}
	d.OnFileHashed.Fire(path, fileHash)
	return nil
}

func (d *Deduplicator) hashFile(path *pathlib.Path) (string, error) {
	file, err := path.Open()
	if err != nil {
		return "", err
	}
	defer file.Close()
	d.hash.Reset()
	if _, err := io.Copy(d.hash, file); err != nil {
		return "", err
	}
	fileHash := d.hash.Sum(nil)
	return hex.EncodeToString(fileHash), nil
}

func (d *Deduplicator) Deduplicate(hash string, duplicateFiles []*pathlib.Path) error {
	d.OnDeduplicationStarted.Fire()
	defer d.OnDeduplicationFinished.Fire()

	// since these files have the same content, has, and size (but not necessarily other metadata like ownership and
	// access permissions), we *should* be able to arbitrarily choose one file as the canonical file and copy it to the
	// canonical directory.
	d.copyToCanonicalDir(duplicateFiles[0], hash)
	// Once copied, all files can be replaced with softlinks to the canonical file.
	d.replaceFilesWithSoftlinks(duplicateFiles, hash)
	d.OnDeduplicated.Fire(duplicateFiles, d.canonicalDir)
	return nil
}

func (d *Deduplicator) copyToCanonicalDir(path *pathlib.Path, hash string) error {
	canonicalPath := d.canonicalDir.Join(hash)
	exists, err := canonicalPath.Exists()
	if err != nil {
		return err
	}
	if exists {
		isFile, err := canonicalPath.IsFile()
		if err != nil {
			return err
		}
		if !isFile {
			return fmt.Errorf("canonical file %s is not a file", canonicalPath.String())
		}
		existingHash, err := d.hashFile(canonicalPath)
		if err != nil {
			return err
		}
		if existingHash != hash {
			return fmt.Errorf("canonical file %s already exists with different hash: %s (expected: %s)",
				canonicalPath.String(), existingHash, hash)
		}
	}
	if d.dryRun {
		slog.Debug("Would move file to canonical directory here, but dry-run is enabled.", "src_file", path, "tgt_file", canonicalPath)
		return nil
	}
	bytesCopied, err := path.Copy(canonicalPath)
	if err != nil {
		return err
	}
	bytesLeft, err := path.Size()
	if err != nil {
		return err
	}
	if bytesCopied != bytesLeft {
		return fmt.Errorf("file %s was not copied exactly: %d bytes copied (expected: %d)",
			path.String(), bytesCopied, bytesLeft)
	}
	err = canonicalPath.Chtimes(time.Unix(1, 0).UTC(), time.Unix(0, 0).UTC())
	if err != nil {
		return fmt.Errorf("failed to change mtime and atime of file %s", path.String())
	}
	return nil
}

func (d *Deduplicator) replaceFilesWithSoftlinks(paths []*pathlib.Path, hash string) error {
	return nil
}
