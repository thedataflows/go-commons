package grep

import (
	"regexp"
)

// Grep searches for query string in a list of paths and if successful, returns a list of matches
//
// Using Andrew Healey's code. See https://healeycodes.com/beating-grep-with-go
func Grep(paths []string, query string, isRegex bool, showLines bool, workers int) ([]string, error) {
	var opts *SearchOptions

	if isRegex {
		r, err := regexp.Compile(query)
		if err != nil {
			return nil, err
		}
		opts = &SearchOptions{
			Kind:   REGEX,
			Lines:  showLines,
			Regex:  r,
			Finder: nil,
		}
	} else {
		opts = &SearchOptions{
			Kind:   LITERAL,
			Lines:  showLines,
			Regex:  nil,
			Finder: MakeStringFinder([]byte(query)),
		}
	}

	Search(paths, opts, &SearchDebug{
		Workers: workers,
	})
	return []string{}, nil
}
