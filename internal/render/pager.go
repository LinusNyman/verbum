package render

import (
	"io"
	"os"
	"os/exec"
	"strings"
)

// Page writes content to stdout, routing through a pager when stdout is a
// terminal. When piped (not a TTY) it writes plain text so verbum composes in
// pipelines. Honours $PAGER; PAGER=cat (or empty on a pipe) means no pager.
func Page(content string, isTTY bool) error {
	if !isTTY {
		_, err := io.WriteString(os.Stdout, content)
		return err
	}
	pager := os.Getenv("PAGER")
	if pager == "" {
		pager = "less -RFX"
	}
	if pager == "cat" {
		_, err := io.WriteString(os.Stdout, content)
		return err
	}
	fields := strings.Fields(pager)
	cmd := exec.Command(fields[0], fields[1:]...)
	cmd.Stdin = strings.NewReader(content)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		// Pager missing/failed — fall back to direct write rather than error out.
		_, werr := io.WriteString(os.Stdout, content)
		return werr
	}
	return nil
}
