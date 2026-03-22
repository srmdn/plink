// migrate imports shortcuts from a Slash SQLite database into plink.
//
// Usage:
//
//	go run ./cmd/migrate --slash-db slash_prod.db --plink-db plink.db
//
// Flags:
//
//	--slash-db   path to Slash's SQLite database file (required)
//	--plink-db   path to plink's SQLite database file (required)
//	--dry-run    print what would be imported without writing anything
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	_ "modernc.org/sqlite"
)

type slashShortcut struct {
	id          int
	name        string
	link        string
	title       string
	description string
	tag         string
	rowStatus   string
	createdTs   int64
	updatedTs   int64
}

func main() {
	slashDB := flag.String("slash-db", "", "path to Slash SQLite database (required)")
	plinkDB := flag.String("plink-db", "", "path to plink SQLite database (required)")
	dryRun := flag.Bool("dry-run", false, "print what would be imported without writing")
	flag.Parse()

	if *slashDB == "" || *plinkDB == "" {
		flag.Usage()
		os.Exit(1)
	}

	src, err := sql.Open("sqlite", *slashDB+"?mode=ro")
	if err != nil {
		log.Fatalf("open slash db: %v", err)
	}
	defer src.Close()

	shortcuts, err := readShortcuts(src)
	if err != nil {
		log.Fatalf("read shortcuts: %v", err)
	}

	fmt.Printf("Found %d shortcuts in Slash\n", len(shortcuts))

	if *dryRun {
		fmt.Println("\n[dry-run] Would import:")
		for _, s := range shortcuts {
			active := 1
			if s.rowStatus == "ARCHIVED" {
				active = 0
			}
			desc := s.description
			if desc == "" {
				desc = strings.TrimSpace(s.title)
			}
			fmt.Printf("  slug=%-30s url=%s  category=%q  active=%d\n", s.name, s.link, s.tag, active)
			_ = desc
		}
		return
	}

	dst, err := sql.Open("sqlite", *plinkDB)
	if err != nil {
		log.Fatalf("open plink db: %v", err)
	}
	defer dst.Close()

	inserted, skipped, err := importShortcuts(dst, shortcuts)
	if err != nil {
		log.Fatalf("import: %v", err)
	}

	fmt.Printf("Done: %d inserted, %d skipped (slug already exists)\n", inserted, skipped)
}

func readShortcuts(db *sql.DB) ([]slashShortcut, error) {
	rows, err := db.Query(`
		SELECT id, name, link, title, description, tag, row_status, created_ts, updated_ts
		FROM shortcut
		ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []slashShortcut
	for rows.Next() {
		var s slashShortcut
		if err := rows.Scan(&s.id, &s.name, &s.link, &s.title, &s.description, &s.tag, &s.rowStatus, &s.createdTs, &s.updatedTs); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func importShortcuts(db *sql.DB, shortcuts []slashShortcut) (inserted, skipped int, err error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO links (slug, url, description, category, active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(slug) DO NOTHING
	`)
	if err != nil {
		return 0, 0, err
	}
	defer stmt.Close()

	for _, s := range shortcuts {
		active := 1
		if s.rowStatus == "ARCHIVED" {
			active = 0
		}

		// prefer description, fall back to title
		desc := strings.TrimSpace(s.description)
		if desc == "" {
			desc = strings.TrimSpace(s.title)
		}

		res, err := stmt.Exec(s.name, s.link, desc, s.tag, active, s.createdTs, s.updatedTs)
		if err != nil {
			return inserted, skipped, fmt.Errorf("insert %q: %w", s.name, err)
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			skipped++
		} else {
			inserted++
		}
	}

	return inserted, skipped, tx.Commit()
}
