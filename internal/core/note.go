package core

import (
	"fmt"
	"strings"
	"time"
)

// NoteDateLayout is the canonical logical-date format for notes and
// other date-only fields (see docs/DATA_MODEL.md): YYYY-MM-DD.
const NoteDateLayout = "2006-01-02"

// Note is a dated reference to a markdown file the user maintains
// outside Orbit. Orbit stores only the absolute path and a logical
// date; it never creates or owns the file. The date feeds work-day
// tracking later, which is why every note carries one.
type Note struct {
	ID          int64
	WorkEntryID string
	// Path is the absolute path to the markdown file. Absolutization
	// happens in the app layer (it depends on the working directory).
	Path string
	// Date is the logical date the note belongs to, in [NoteDateLayout].
	Date      string
	CreatedAt time.Time
}

// NormalizeNoteDate validates a logical note date. An empty raw value
// defaults to today (UTC); a non-empty value must be a valid
// [NoteDateLayout] date. The canonical YYYY-MM-DD string is returned.
func NormalizeNoteDate(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return time.Now().UTC().Format(NoteDateLayout), nil
	}
	d, err := time.Parse(NoteDateLayout, s)
	if err != nil {
		return "", fmt.Errorf("invalid note date %q: want YYYY-MM-DD", raw)
	}
	return d.Format(NoteDateLayout), nil
}
