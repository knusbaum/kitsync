package kitsync

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/lib/pq"
)

type psqlIndex struct {
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

func newPsqlIndex(host string, port int, user, password, dbname string) (*psqlIndex, error) {
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

	return &psqlIndex{db: db}, nil
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

func (i *psqlIndex) Iter() (IndexIterator, error) {
	rs, err := i.db.Query("SELECT hash FROM objects ORDER BY hash;")
	if err != nil {
		return nil, err
	}
	return &rowIterator{rs: rs}, nil
}

func (i *psqlIndex) Present(id string) bool {
	var ohash string
	if err := i.db.QueryRow("SELECT hash FROM objects WHERE hash = $1;", id).Scan(&ohash); err != nil {
		if err == sql.ErrNoRows {
			return false
		}
	}
	return ohash == id
}

func (i *psqlIndex) objectOID(id string) (int, error) {
	var oid int
	// Query for a value based on a single row.
	if err := i.db.QueryRow("SELECT id FROM objects WHERE hash = $1;", id).Scan(&oid); err != nil {
		if err == sql.ErrNoRows {
			_, err := i.db.Exec("INSERT INTO objects (hash) VALUES ($1);", id)
			if err != nil {
				return 0, fmt.Errorf("Failed to insert hash into objects: %w", err)
			}
			return i.objectOID(id)
		}
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

func (i *psqlIndex) Close() error {
	return i.db.Close()
}

var keyedSearchQuery = `SELECT objects.hash FROM objects
JOIN tags ON tags.oid = objects.id
WHERE tags.key = $1 AND tags.value ilike $2
ORDER BY objects.hash`

var keylessSearchQuery = `SELECT objects.hash FROM objects
JOIN keyless ON keyless.oid = objects.id
WHERE keyless.tag ilike $1
ORDER BY objects.hash`

func (i *psqlIndex) SearchTag(k, v string) (IndexIterator, error) {
	if k != "" {

		rs, err := i.db.Query(keyedSearchQuery, k, "%"+v+"%")
		if err != nil {
			return nil, err
		}
		return &rowIterator{rs: rs}, nil
	} else {
		rs, err := i.db.Query(keylessSearchQuery, "%"+v+"%")
		if err != nil {
			return nil, err
		}
		return &rowIterator{rs: rs}, nil
	}
}
