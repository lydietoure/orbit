//go:build ci

// genschema applies SQL migration files to an in-memory SQLite database and
// writes a normalised schema dump — suitable for golden-file testing or
// one-off inspection.
//
// Run from the repository root:
//
//	go run -tags ci ./cmd/genschema/
//	go run -tags ci ./cmd/genschema/ -out /tmp/current.sql
//	go run -tags ci ./cmd/genschema/ -migrations "0000_v0.1.0.sql" -out /tmp/v010.sql
//	go run -tags ci ./cmd/genschema/ -sql /tmp/released.sql -out /tmp/released_norm.sql
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	orbitdb "github.com/lydietoure/orbit/internal/db"
	_ "modernc.org/sqlite"
)

func main() {
	out := flag.String("out", "", "path to write the generated schema file (required)")
	migrations := flag.String("migrations", "", "comma- or space-separated migration filenames (base names only); defaults to all files in the migrations dir")
	sqlFile := flag.String("sql", "", "path to an arbitrary SQL file to apply directly (mutually exclusive with -migrations)")
	flag.Parse()

	if *out == "" {
		fatalf("-out is required")
	}
	if *sqlFile != "" && *migrations != "" {
		fatalf("-sql and -migrations are mutually exclusive")
	}

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		fatalf("open in-memory db: %v", err)
	}
	defer db.Close()

	migrationsDir := filepath.Join("internal", "db", "migrations")

	switch {
	case *sqlFile != "":
		applyFile(db, *sqlFile)

	case *migrations != "":
		sep := func(r rune) bool { return r == ',' || r == ' ' }
		var files []string
		for _, name := range strings.FieldsFunc(*migrations, sep) {
			if name = strings.TrimSpace(name); name != "" {
				files = append(files, filepath.Join(migrationsDir, name))
			}
		}
		if len(files) == 0 {
			fatalf("-migrations was set but contained no file names")
		}
		for _, f := range files {
			applyFile(db, f)
		}

	default:
		entries, err := os.ReadDir(migrationsDir)
		if err != nil {
			fatalf("read migrations dir %s: %v", migrationsDir, err)
		}
		var files []string
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
				files = append(files, filepath.Join(migrationsDir, e.Name()))
			}
		}
		sort.Strings(files)
		if len(files) == 0 {
			fatalf("no .sql files found in %s", migrationsDir)
		}
		for _, f := range files {
			applyFile(db, f)
		}
	}

	lines, err := orbitdb.DumpSchema(db)
	if err != nil {
		fatalf("dump schema: %v", err)
	}
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(*out, []byte(content), 0o644); err != nil {
		fatalf("write %s: %v", *out, err)
	}
	fmt.Printf("wrote %s\n", *out)
}

func applyFile(db *sql.DB, path string) {
	content, err := os.ReadFile(path)
	if err != nil {
		fatalf("read %s: %v", path, err)
	}
	if _, err := db.Exec(string(content)); err != nil {
		fatalf("exec %s: %v", filepath.Base(path), err)
	}
	fmt.Printf("applied %s\n", filepath.Base(path))
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "genschema: "+format+"\n", args...)
	os.Exit(1)
}
