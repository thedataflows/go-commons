/**
* Use table-driven tests in Go to define multiple test cases for each function. blog.logrocket.com
* Design tests based on the requirements or functionality of the system. usersnap.com
* Use parametrized test fixtures to test different implementations of the same interface. stackoverflow.com
**/

package search

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"syscall"
	"testing"
)

type testCase struct {
	finder       Finder
	filePath     string
	expected     []Result
	hasException bool
}

var testCases = []testCase{
	{
		finder:   &JustLister{OpenFile: true},
		filePath: "nonexistent.txt",
		expected: []Result{
			{
				FilePath: "nonexistent.txt",
				Err:      &os.PathError{Op: "open", Path: "nonexistent.txt", Err: syscall.ENOENT},
			},
		},
		hasException: true,
	},
	// Add more test cases for SimpleFinder here.
	{
		finder:   &RegexFinder{Pattern: regexp.MustCompile(`\d`)},
		filePath: "testfile.txt",
		expected: []Result{
			{
				Line:     "123",
				LineNum:  1,
				FilePath: "testfile.txt",
			},
		},
		hasException: false,
	},
	// Add more test cases for RegexFinder here.
	{
		finder:   &TextFinder{Text: []byte("abc")},
		filePath: "testfile.txt",
		expected: []Result{
			{
				Line:     "abc",
				LineNum:  2,
				FilePath: "testfile.txt",
			},
		},
		hasException: false,
	},
	{
		finder:       &TextFinder{Text: []byte("NotMatched")},
		filePath:     "testfile.txt",
		expected:     []Result{},
		hasException: false,
	},
	// Add more test cases for TextFinder here.
}

func TestProcessFile(t *testing.T) {
	ctx := context.Background()
	resultsChan := make(chan *Results)

	for _, tc := range testCases {
		go tc.finder.ProcessFile(ctx, tc.filePath, resultsChan)
		results := <-resultsChan

		if len(results.Results) == 0 && len(tc.expected) != 0 {
			if tc.hasException {
				t.Errorf("Expected an error, but got no results")
			} else {
				t.Errorf("Expected results for %+v, got none", tc.finder)
			}
		}

		for i, result := range results.Results {
			if result.Line != tc.expected[i].Line || result.LineNum != tc.expected[i].LineNum || result.FilePath != tc.expected[i].FilePath || fmt.Sprintf("%v", result.Err) != fmt.Sprintf("%v", tc.expected[i].Err) {
				t.Errorf("Expected result %+v, but got %+v", tc.expected[i], result)
			}
		}
	}
}
