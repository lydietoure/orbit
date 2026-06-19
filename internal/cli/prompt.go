package cli

// Tiny interactive-prompt helper for destructive commands like
// `work delete`. Kept in its own file because it's behavior-free
// glue: read a line, normalize, compare to "y"/"yes".

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// confirm asks question followed by " [y/N]: " on out and reads
// one line from in. Returns true iff the trimmed, lower-cased
// answer is exactly "y" or "yes". Anything else — blank input,
// "n", "no", random text, or EOF — counts as no, matching POSIX
// convention for capitalized-default prompts.
//
// The prompt is written to out (caller's choice; typically
// cmd.ErrOrStderr() so it stays out of any captured stdout
// payload). I/O errors on either side surface as the returned
// error so the caller can decide whether to retry or bail.
func confirm(in io.Reader, out io.Writer, question string) (bool, error) {
	if _, err := fmt.Fprintf(out, "%s [y/N]: ", question); err != nil {
		return false, err
	}
	reader := bufio.NewReader(in)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}
	answer := strings.TrimSpace(strings.ToLower(line))
	return answer == "y" || answer == "yes", nil
}
