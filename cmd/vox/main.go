// Command vox is an offline Wiktionary CLI dictionary.
package main

import (
	"os"

	"github.com/linusnyman/vox/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:]))
}
