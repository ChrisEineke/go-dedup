package dedup

import (
	"database/sql"
	"os"

	"github.com/ChrisEineke/go-events"
	"github.com/chigopher/pathlib"
	_ "modernc.org/sqlite"
)

type SqliteMode int

const (
	SqliteInMemory = iota
	SqliteTempFile = iota
)

type SqliteDatabase struct {
	mode            SqliteMode
	filename        string
	conn            *sql.DB
	OnFileHashAdded events.E
}

func NewSqliteDatabase(mode SqliteMode) *SqliteDatabase {
	db := &SqliteDatabase{
		mode:            mode,
		filename:        "",
		conn:            nil,
		OnFileHashAdded: events.E{N: EvtFileHashAdded},
	}
	_ = Database(db)
	return db
}

func (db *SqliteDatabase) Open() error {
	switch db.mode {
	case SqliteInMemory:
		db.filename = "file::memory:?cache=shared"
	case SqliteTempFile:
		tempfile, err := os.CreateTemp("", "")
		if err != nil {
			return err
		}
		db.filename = tempfile.Name()
	}
	conn, err := sql.Open("sqlite", db.filename)
	if err != nil {
		return err
	}
	db.conn = conn
	db.conn.SetMaxIdleConns(1) // so the in-memory database won't get deleted
	db.conn.SetConnMaxLifetime(MaxDuration)
	return db.initSchema()
}

func (db *SqliteDatabase) initSchema() error {
	var query string
	var rows *sql.Rows
	var err error

	query = "CREATE TABLE IF NOT EXISTS file_hashes(file text, hash text)"
	rows, err = db.conn.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()

	query = "CREATE UNIQUE INDEX IF NOT EXISTS file_hashes_unique_file ON file_hashes(file)"
	rows, err = db.conn.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()

	query = "CREATE INDEX IF NOT EXISTS file_hashes_unique_hash ON file_hashes(hash)"
	rows, err = db.conn.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()

	return nil
}

func (db *SqliteDatabase) Close() error {
	if err := db.conn.Close(); err != nil {
		return err
	}
	if err := os.Remove(db.filename); err != nil {
		return err
	}
	db.filename = ""
	return nil
}

func (db *SqliteDatabase) AddFileHash(filepath *pathlib.Path, hash string) error {
	// TODO: handle result?
	query := `
	BEGIN IMMEDIATE;
		INSERT INTO file_hashes VALUES(?, ?);
	COMMIT;`
	_, err := db.conn.Exec(query, filepath.String(), hash)
	if err != nil {
		return err
	}
	db.OnFileHashAdded.Fire(filepath, hash)
	return nil
}

func (db *SqliteDatabase) FindDuplicateHashes() ([]*DuplicateHash, error) {
	dups := []*DuplicateHash{}
	query := `
	SELECT hash
	FROM file_hashes
	GROUP BY hash
	HAVING COUNT(hash) > 2
	ORDER BY COUNT(hash) DESC
	`
	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		dup := DuplicateHash{}
		if err := rows.Scan(&dup.Hash); err != nil {
			return nil, err
		}
		dup.Files, err = db.findFilesWithHash(dup.Hash)
		if err != nil {
			return nil, err
		}
		dups = append(dups, &dup)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return dups, nil
}

func (db *SqliteDatabase) findFilesWithHash(hash string) ([]*pathlib.Path, error) {
	fileNames := []*pathlib.Path{}
	query := `
	SELECT file FROM file_hashes
	WHERE hash = ?
	ORDER BY file ASC
	`
	rows, err := db.conn.Query(query, hash)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var file string
		if err := rows.Scan(&file); err != nil {
			return nil, err
		}
		fileNames = append(fileNames, pathlib.NewPath(file))
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return fileNames, nil
}
