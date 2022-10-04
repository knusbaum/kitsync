package kitsync

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/lib/pq"
)

type psqlIndex struct {
	Storage
	db *sql.DB
}

var createObjects = `CREATE TABLE IF NOT EXISTS objects (
	id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
	hash TEXT UNIQUE
);`

var createTags = `CREATE TABLE IF NOT EXISTS tags (
	oid INTEGER, 
	key TEXT, 
	value TEXT, 
	PRIMARY KEY (oid, key),
	CONSTRAINT fk_oid FOREIGN KEY (oid) 
	REFERENCES objects(id)
	ON UPDATE CASCADE
	ON DELETE CASCADE
);`

var createKeyless = `CREATE TABLE IF NOT EXISTS keyless (
	oid INTEGER,
	tag TEXT,
	PRIMARY KEY (oid, tag),
	CONSTRAINT fk_oid FOREIGN KEY (oid) 
	REFERENCES objects(id)
	ON UPDATE CASCADE
	ON DELETE CASCADE
);`

func NewPsqlIndex(s Storage, host string, port int, user, password, dbname string) (*psqlIndex, error) {
	// 	//psqlconn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", host, port, user, password, dbname)
	// 	psqlconn := fmt.Sprintf("host=%s port=%d user=%s password=%s sslmode=disable", host, port, user, password)
	// 	// open database
	// 	db, err := sql.Open("postgres", psqlconn)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	//
	// 	fmt.Printf("Creating dbname.\n")
	// 	_, err = db.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s;\n", dbname))
	// 	if err != nil {
	// 		return nil, err
	// 	}

	psqlconn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", host, port, user, password, dbname)
	db, err := sql.Open("postgres", psqlconn)
	if err != nil {
		return nil, fmt.Errorf("Failed to open %s: %w. Please make sure database %s exists.", psqlconn, err, dbname)
	}

	fmt.Printf("Creating objects.\n")
	_, err = db.Exec(createObjects)
	if err != nil {
		return nil, err
	}

	fmt.Printf("Creating tags.\n")
	_, err = db.Exec(createTags)
	if err != nil {
		return nil, err
	}

	fmt.Printf("Creating keyless.\n")
	_, err = db.Exec(createKeyless)
	if err != nil {
		return nil, err
	}

	return &psqlIndex{Storage: s, db: db}, nil
}

type rowIterator struct {
	rs *sql.Rows
}

func (i *rowIterator) Next() (string, error) {
	if !i.rs.Next() {
		return "", itDone
	}
	var ret string
	err := i.rs.Scan(&ret)
	if err != nil {
		return "", err
	}
	return ret, nil
}

func (i *rowIterator) Close() error {
	return i.rs.Close()
}

func (i *psqlIndex) IDs() (Iterator, error) {
	rs, err := i.db.Query("SELECT hash FROM objects ORDER BY hash;")
	if err != nil {
		return nil, err
	}
	return &rowIterator{rs: rs}, nil
}

func (i *psqlIndex) Indexed(id string) bool {
	var ohash string
	if err := i.db.QueryRow("SELECT hash FROM objects WHERE hash = $1;", id).Scan(&ohash); err != nil {
		if err == sql.ErrNoRows {
			return false
		}
	}
	return ohash == id
}

func (i *psqlIndex) Sync() error {
	fmt.Printf("Syncing Index.\n")
	defer fmt.Printf("Done syncing index.\n")
	if sync, ok := i.Storage.(Syncer); ok {
		fmt.Printf("Index syncing underlying storage.\n")
		err := sync.Sync()
		if err != nil {
			return err
		}
		fmt.Printf("Done syncing underlying storage.\n")
	}
	it, err := i.Iter()
	if err != nil {
		return err
	}
	var id string
	for id, err = it.Next(); err == nil; id, err = it.Next() {
		if !i.Indexed(id) {
			o, ok := i.Get(id)
			if !ok {
				return fmt.Errorf("Storage claims to have id %s, but Get does not return it.\n", id)
			}
			fmt.Printf("Adding %s to index.\n", o.ID())
			if err := i.Add(o); err != nil {
				return err
			}
		}
	}
	return nil
}

func (i *psqlIndex) Reindex() error {
	if _, err := i.db.Exec("DELETE FROM objects;"); err != nil {
		return err
	}
	return i.Sync()
}

func (i *psqlIndex) Put(o Object) error {
	err := i.Storage.Put(o)
	if err != nil {
		return err
	}
	return i.Add(o)
}

func (i *psqlIndex) objectOID(id string) (int, error) {
	if _, err := i.db.Exec("DELETE FROM objects WHERE hash = $1;", id); err != nil {
		return 0, err
	}

	_, err := i.db.Exec("INSERT INTO objects (hash) VALUES ($1);", id)
	if err != nil {
		return 0, fmt.Errorf("Failed to insert hash into objects: %w", err)
	}

	var oid int
	if err := i.db.QueryRow("SELECT id FROM objects WHERE hash = $1;", id).Scan(&oid); err != nil {
		return 0, fmt.Errorf("Failed to select ID from objects by hash: %w", err)
	}
	return oid, nil
}

func (i *psqlIndex) Add(o Object) error {
	tags, err := o.Tags()
	if err != nil {
		return err
	}

	oid, err := i.objectOID(o.ID())
	if err != nil {
		return err
	}

	for k, v := range tags {
		k = strings.TrimSpace(k)
		if k == "" {
			for _, tag := range strings.Split(v, " ") {
				_, err := i.db.Exec("INSERT INTO keyless (oid, tag) values ($1, $2)", oid, strings.TrimSpace(tag))
				if err != nil {
					return err
				}
			}
		} else {
			_, err := i.db.Exec("INSERT INTO tags (oid, key, value) values ($1, $2, $3)", oid, strings.TrimSpace(k), strings.TrimSpace(v))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (i *psqlIndex) Delete(id string) error {
	err := i.Storage.Delete(id)
	if err != nil {
		return err
	}
	if _, err := i.db.Exec("DELETE FROM objects WHERE hash = $1;", id); err != nil {
		return err
	}
	return nil
}

func (i *psqlIndex) Close() error {
	return i.db.Close()
}

var keyedSearchQuery = `SELECT objects.hash FROM objects
JOIN tags ON tags.oid = objects.id
WHERE tags.key = $1 AND tags.value ilike $2
ORDER BY objects.hash`

var keylessSearchQuery = `SELECT DISTINCT objects.hash FROM objects
LEFT JOIN keyless ON keyless.oid = objects.id
LEFT JOIN tags on tags.oid = objects.id
WHERE keyless.tag ilike $1 OR tags.value ilike $1
ORDER BY objects.hash`

func (i *psqlIndex) SearchTag(k, v string) (Iterator, error) {
	if k != "" {
		rs, err := i.db.Query(keyedSearchQuery, k, "%"+v+"%")
		if err != nil {
			return nil, err
		}
		return &rowIterator{rs: rs}, nil
	} else {
		fmt.Printf("KEYLESS SEARCH QUERY\n")
		rs, err := i.db.Query(keylessSearchQuery, "%"+v+"%")
		if err != nil {
			return nil, err
		}
		return &rowIterator{rs: rs}, nil
	}
}

func (i *psqlIndex) Keys() (Iterator, error) {
	rs, err := i.db.Query(`SELECT DISTINCT key FROM tags ORDER BY key;`)
	if err != nil {
		return nil, err
	}
	return &rowIterator{rs: rs}, nil
}

func (i *psqlIndex) Tags() (Iterator, error) {
	rs, err := i.db.Query(`SELECT DISTINCT tag FROM keyless ORDER BY tag;`)
	if err != nil {
		return nil, err
	}
	return &rowIterator{rs: rs}, nil
}

func (i *psqlIndex) Get(id string) (Object, bool) {
	o, ok := i.Storage.Get(id)
	if ok {
		return &iob{o, i}, ok
	}
	return nil, ok
}

type iob struct {
	Object
	i *psqlIndex
}

func (i *iob) AddTag(k, v string) error {
	err := i.Object.AddTag(k, v)
	if err != nil {
		return err
	}
	return i.i.Add(i.Object)
}

func (i *iob) DelTag(k string) error {
	err := i.Object.DelTag(k)
	if err != nil {
		return err
	}
	return i.i.Add(i.Object)
}
