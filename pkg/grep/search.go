package grep

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
)

type SearchDebug struct {
	Workers int
}

const (
	LITERAL = iota
	REGEX
)

type SearchOptions struct {
	Kind   int
	Lines  bool
	Regex  *regexp.Regexp
	Finder *stringFinder
}

type SearchResult struct {
	Path  string
	Match string
	Line  int
	Error error
}

// type SearchJob struct {
// 	Path    string
// 	Options *SearchOptions
// }

const MAX_WORKERS = 128

var (
	done = make(chan struct{})
	sema = make(chan struct{}, MAX_WORKERS) // concurrency-limiting counting semaphore
)

func cancelled() bool {
	select {
	case <-done:
		return true
	default:
		return false
	}
}

func Search(paths []string, opts *SearchOptions, debug *SearchDebug) {
	// Cancel when input is detected.
	go func() {
		os.Stdin.Read(make([]byte, 1)) // read a single byte
		close(done)
	}()

	var wg sync.WaitGroup
	searchJobs := make(chan *[]SearchResult)
	// for w := 0; w < debug.Workers; w++ {
	// 	go searchWorker(searchJobs, &wg)
	// }
	for _, path := range paths {
		dirTraversal(path, opts, searchJobs, &wg)
	}
	go func() {
		wg.Wait()
		close(searchJobs)
	}()

loop:
	//!+3
	for {
		select {
		case <-done:
			// Drain other channels to allow existing goroutines to finish.
			for range searchJobs {
				// Do nothing.
			}
			return
		case size, ok := <-fileSizes:
			// ...
			//!-3
			if !ok {
				break loop // fileSizes was closed
			}
			nfiles++
			nbytes += size
		case <-tick:
			printDiskUsage(nfiles, nbytes)
		}
	}
	printDiskUsage(nfiles, nbytes) // final totals
}

func dirTraversal(path string, opts *SearchOptions, searchJobs chan *[]SearchResult, wg *sync.WaitGroup) {
	select {
	case sema <- struct{}{}: // acquire token
	case <-done:
		return // cancelled
	}
	defer func() { <-sema }() // release token

	sr :=

	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("couldn't lstat path %s: %s", path, err)
	}

	if !info.IsDir() {
		wg.Add(1)
		searchJobs <- &SearchJob{
			path,
			opts,
			&SearchResult{},
		}
		return nil
	}

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("couldn't open path %s: %s", path, err)
	}
	dirNames, err := f.Readdirnames(-1)
	if err != nil {
		return fmt.Errorf("couldn't read dir names for path %s: %s", path, err)
	}

	for _, deeperPath := range dirNames {
		if err := dirTraversal(filepath.Join(path, deeperPath), opts, searchJobs, wg); err != nil {
			return err
		}
	}

	return nil
}

func searchWorker(jobs chan *SearchJob, wg *sync.WaitGroup) error {
	for job := range jobs {
		defer wg.Done()
		f, err := os.Open(job.Path)
		if err != nil {
			return fmt.Errorf("couldn't open path %s: %s", job.Path, err)
		}

		scanner := bufio.NewScanner(f)
		isBinary := false

		line := 1
		for scanner.Scan() {
			text := scanner.Bytes()

			// Check the first buffer for NUL
			if line == 1 {
				isBinary = bytes.IndexByte(text, 0) != -1
			}

			if job.Options.Kind == LITERAL {
				if job.Options.Finder.next(text) != -1 {
					if isBinary {
						fmt.Printf("Binary file %s matches\n", job.Path)
						break
					} else if job.Options.Lines {
						fmt.Printf("%s:%d %s\n", job.Path, line, text)
					} else {
						fmt.Printf("%s %s\n", job.Path, text)
					}
				}
			} else if job.Options.Kind == REGEX {
				if job.Options.Regex.Find(scanner.Bytes()) != nil {
					if isBinary {
						fmt.Printf("Binary file %s matches\n", job.Path)
						break
					} else if job.Options.Lines {
						fmt.Printf("%s:%d %s\n", job.Path, line, text)
					} else {
						fmt.Printf("%s %s\n", job.Path, text)
					}
				}
			}
			line++
		}

	}
	return nil
}

// Below, is Go's internal Boyer-Moore string search algorithm, it has been
// modified to use []byte instead of string to reduce allocations.

// https://go.googlesource.com/go/+/go1.18.1/src/strings/search.go
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// stringFinder efficiently finds strings in a source text. It's implemented
// using the Boyer-Moore string search algorithm:
// https://en.wikipedia.org/wiki/Boyer-Moore_string_search_algorithm
// https://www.cs.utexas.edu/~moore/publications/fstrpos.pdf (note: this aged
// document uses 1-based indexing)
type stringFinder struct {
	// pattern is the string that we are searching for in the text.
	pattern []byte

	// badCharSkip[b] contains the distance between the last byte of pattern
	// and the rightmost occurrence of b in pattern. If b is not in pattern,
	// badCharSkip[b] is len(pattern).
	//
	// Whenever a mismatch is found with byte b in the text, we can safely
	// shift the matching frame at least badCharSkip[b] until the next time
	// the matching char could be in alignment.
	badCharSkip [256]int

	// goodSuffixSkip[i] defines how far we can shift the matching frame given
	// that the suffix pattern[i+1:] matches, but the byte pattern[i] does
	// not. There are two cases to consider:
	//
	// 1. The matched suffix occurs elsewhere in pattern (with a different
	// byte preceding it that we might possibly match). In this case, we can
	// shift the matching frame to align with the next suffix chunk. For
	// example, the pattern "mississi" has the suffix "issi" next occurring
	// (in right-to-left order) at index 1, so goodSuffixSkip[3] ==
	// shift+len(suffix) == 3+4 == 7.
	//
	// 2. If the matched suffix does not occur elsewhere in pattern, then the
	// matching frame may share part of its prefix with the end of the
	// matching suffix. In this case, goodSuffixSkip[i] will contain how far
	// to shift the frame to align this portion of the prefix to the
	// suffix. For example, in the pattern "abcxxxabc", when the first
	// mismatch from the back is found to be in position 3, the matching
	// suffix "xxabc" is not found elsewhere in the pattern. However, its
	// rightmost "abc" (at position 6) is a prefix of the whole pattern, so
	// goodSuffixSkip[3] == shift+len(suffix) == 6+5 == 11.
	goodSuffixSkip []int
}

func MakeStringFinder(pattern []byte) *stringFinder {
	f := &stringFinder{
		pattern:        pattern,
		goodSuffixSkip: make([]int, len(pattern)),
	}
	// last is the index of the last character in the pattern.
	last := len(pattern) - 1

	// Build bad character table.
	// Bytes not in the pattern can skip one pattern's length.
	for i := range f.badCharSkip {
		f.badCharSkip[i] = len(pattern)
	}
	// The loop condition is < instead of <= so that the last byte does not
	// have a zero distance to itself. Finding this byte out of place implies
	// that it is not in the last position.
	for i := 0; i < last; i++ {
		f.badCharSkip[pattern[i]] = last - i
	}

	// Build good suffix table.
	// First pass: set each value to the next index which starts a prefix of
	// pattern.
	lastPrefix := last
	for i := last; i >= 0; i-- {
		if bytes.HasPrefix(pattern, pattern[i+1:]) {
			lastPrefix = i + 1
		}
		// lastPrefix is the shift, and (last-i) is len(suffix).
		f.goodSuffixSkip[i] = lastPrefix + last - i
	}
	// Second pass: find repeats of pattern's suffix starting from the front.
	for i := 0; i < last; i++ {
		lenSuffix := longestCommonSuffix(pattern, pattern[1:i+1])
		if pattern[i-lenSuffix] != pattern[last-lenSuffix] {
			// (last-i) is the shift, and lenSuffix is len(suffix).
			f.goodSuffixSkip[last-lenSuffix] = lenSuffix + last - i
		}
	}

	return f
}

func longestCommonSuffix(a, b []byte) (i int) {
	for ; i < len(a) && i < len(b); i++ {
		if a[len(a)-1-i] != b[len(b)-1-i] {
			break
		}
	}
	return
}

// next returns the index in text of the first occurrence of the pattern. If
// the pattern is not found, it returns -1.
func (f *stringFinder) next(text []byte) int {
	i := len(f.pattern) - 1
	for i < len(text) {
		// Compare backwards from the end until the first unmatching character.
		j := len(f.pattern) - 1
		for j >= 0 && text[i] == f.pattern[j] {
			i--
			j--
		}
		if j < 0 {
			return i + 1 // match
		}
		i += max(f.badCharSkip[text[i]], f.goodSuffixSkip[j])
	}
	return -1
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
