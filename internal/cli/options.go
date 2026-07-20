package cli

import (
	"fmt"
	"strings"
)

// Options is the parsed command line for a lookup invocation.
type Options struct {
	Verbose bool
	Etym    bool
	Quirks  bool
	Fuzzy   bool // -k : spell mode (candidates are the output)
	Reverse bool // -r : query is a definition, search glosses
	JSON    bool
	Trans   []string // -t : translation target codes
	Langs   []string // -l : restrict source languages
	Help    bool
	Version bool
	Args    []string // positional query tokens
}

// parse implements a small getopt-style parser: bundled boolean shorts
// (-eq == -e -q), value flags -t/-l (attached or next arg), and long flags.
func parse(argv []string) (Options, error) {
	var o Options
	i := 0
	for i < len(argv) {
		a := argv[i]
		switch {
		case a == "--":
			o.Args = append(o.Args, argv[i+1:]...)
			return o, nil
		case a == "--help" || a == "-h":
			o.Help = true
		case a == "--version":
			o.Version = true
		case a == "--json":
			o.JSON = true
		case strings.HasPrefix(a, "--trans="):
			o.Trans = append(o.Trans, strings.TrimPrefix(a, "--trans="))
		case strings.HasPrefix(a, "--lang="):
			o.Langs = append(o.Langs, strings.TrimPrefix(a, "--lang="))
		case a == "--trans", a == "--lang":
			val, ni, err := value(argv, i, a)
			if err != nil {
				return o, err
			}
			if a == "--trans" {
				o.Trans = append(o.Trans, val)
			} else {
				o.Langs = append(o.Langs, val)
			}
			i = ni
		case len(a) > 1 && a[0] == '-' && a[1] != '-':
			ni, err := shorts(&o, argv, i)
			if err != nil {
				return o, err
			}
			i = ni
		default:
			o.Args = append(o.Args, a)
		}
		i++
	}
	return o, nil
}

// shorts consumes a bundle like -v, -eq, -tde, or -t de. Returns the index of
// the last consumed argv element.
func shorts(o *Options, argv []string, i int) (int, error) {
	bundle := argv[i][1:]
	for j := 0; j < len(bundle); j++ {
		switch bundle[j] {
		case 'v':
			o.Verbose = true
		case 'e':
			o.Etym = true
		case 'q':
			o.Quirks = true
		case 'k':
			o.Fuzzy = true
		case 'r':
			o.Reverse = true
		case 't', 'l':
			// value flag: rest of bundle, else next arg.
			flag := bundle[j]
			val := bundle[j+1:]
			if val == "" {
				if i+1 >= len(argv) {
					return i, fmt.Errorf("flag -%c needs a language code", flag)
				}
				i++
				val = argv[i]
			}
			if flag == 't' {
				o.Trans = append(o.Trans, val)
			} else {
				o.Langs = append(o.Langs, val)
			}
			return i, nil
		default:
			return i, fmt.Errorf("unknown flag -%c", bundle[j])
		}
	}
	return i, nil
}

func value(argv []string, i int, name string) (string, int, error) {
	if i+1 >= len(argv) {
		return "", i, fmt.Errorf("flag %s needs a value", name)
	}
	return argv[i+1], i + 1, nil
}

func (o Options) query() string { return strings.Join(o.Args, " ") }
