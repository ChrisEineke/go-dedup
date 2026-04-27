package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/ChrisEineke/go-dedup/dedup"
	"github.com/ChrisEineke/go-events"
	"github.com/chigopher/pathlib"
)

func main() {
	if err := run(); err != nil {
		exit(err, 1)
	} else {
		exit(nil, 0)
	}
}

func exit(err error, code int) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "dedup: %+v\n", err)
	}
	os.Exit(code)
}

func run() error {
	var err error

	var canonicalDir string
	var dryRun bool
	var verbose bool

	flag.StringVar(&canonicalDir, "canonical-dir", "", "Where to store canonical files. (required)")
	flag.BoolVar(&dryRun, "dry-run", false, "Only print what's going to happen; don't actually do it. (optional)")
	flag.BoolVar(&verbose, "verbose", false, "Prints verbose information during processing. (optional)")
	flag.Parse()

	if verbose {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}
	if flag.NArg() == 0 {
		flag.Usage()
		return fmt.Errorf("missing one or more directories to deduplicate")
	}
	canonicalPath, err := pathlib.NewPath(canonicalDir).ResolveAll()
	if err != nil {
		flag.Usage()
		return fmt.Errorf("failed to resolve canonical dir %s", canonicalPath.String())
	}
	exists, err := canonicalPath.Exists()
	if !exists {
		flag.Usage()
		return fmt.Errorf("canonical dir %s does not exist", canonicalPath.String())
	}

	deduplicator := dedup.NewDeduplicator(canonicalPath, dryRun)
	db := dedup.NewInMemDatabase()
	// db := dedup.NewSqliteDatabase(dedup.SqliteTempFile)

	deduplicator.OnFileHashed.On(func(filePath *pathlib.Path, fileHash string) {
		db.AddFileHash(filePath, fileHash)
	}, events.Async())
	deduplicator.OnFileHashingFinished.On(func() {
		duplicateHashes, err := db.FindDuplicateHashes()
		if err != nil {
			exit(err, 1)
		}
		for _, duplicateHash := range duplicateHashes {
			deduplicator.Deduplicate(duplicateHash.Hash, duplicateHash.Files)
		}
	})

	if verbose {
		loggerware := events.Logger(os.Stdout, "")
		db.OnFileHashAdded.Use(loggerware)
		deduplicator.OnDeduplicated.Use(loggerware)
		deduplicator.OnFileHashingStarted.Use(loggerware)
		deduplicator.OnFileHashed.Use(loggerware)
		deduplicator.OnFileHashingFinished.Use(loggerware)
	}

	err = db.Open()
	if err != nil {
		return err
	}
	defer db.Close()

	dirs := []*pathlib.Path{}
	for _, arg := range flag.Args() {
		dirs = append(dirs, pathlib.NewPath(arg))
	}
	deduplicator.HashFiles(dirs)
	return nil
}
