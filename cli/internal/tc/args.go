package tc

import (
	"fmt"
	"strings"
)

// ParsedArgs holds the parsed command name and its --key value options.
// Option keys are matched case-insensitively.
type ParsedArgs struct {
	Command string
	options map[string]string
}

// helpAliases are the first-argument forms that map to the "help" command,
// matched case-insensitively (help, -h, --help).
var helpAliases = map[string]struct{}{
	"help":   {},
	"-h":     {},
	"--help": {},
}

// ParseArgs parses the raw argument list into a command and its options.
//
// With no arguments, or when the first argument is a help alias, the command is
// "help". Otherwise the first argument is the command and the remainder are read
// as --key value pairs from index 1 onwards in steps of two; a trailing flag
// with no value is ignored.
func ParseArgs(args []string) *ParsedArgs {
	if len(args) == 0 {
		return newParsedArgs("help")
	}
	if _, ok := helpAliases[strings.ToLower(args[0])]; ok {
		return newParsedArgs("help")
	}

	parsed := newParsedArgs(args[0])
	for i := 1; i < len(args)-1; i += 2 {
		key := args[i]
		value := args[i+1]
		if strings.HasPrefix(key, "--") {
			parsed.options[strings.ToLower(key[2:])] = value
		}
	}
	return parsed
}

func newParsedArgs(command string) *ParsedArgs {
	return &ParsedArgs{Command: command, options: map[string]string{}}
}

// GetRequired returns the value for name, or an error if it is absent.
func (p *ParsedArgs) GetRequired(name string) (string, error) {
	if value, ok := p.options[strings.ToLower(name)]; ok {
		return value, nil
	}
	return "", fmt.Errorf("missing required argument: --%s", name)
}

// GetOptional returns the value for name and whether it was present.
func (p *ParsedArgs) GetOptional(name string) (string, bool) {
	value, ok := p.options[strings.ToLower(name)]
	return value, ok
}
