package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// locates this directory's parent `.git` directory and returns it, or an error
// if no parent `.git` directory could be found.
func gitPath() (string, error) {
	// start at the current directory
	cur, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// clean up the path and ensure it's absolute so we can traverse all the way
	// to the root directory if necessary.
	cur, err = filepath.Abs(filepath.Clean(cur))
	if err != nil {
		return "", err
	}

	// walk our way up the directory tree, attempting to find a `.git` directory
	const gitDirectoryName = ".git"
	for cur != "/" {
		// list all this directory's children
		children, err := ioutil.ReadDir(cur)
		if err != nil {
			return "", err
		}

		// look for a `.git` directory in the children
		for _, info := range children {
			name := info.Name()

			// if we find a directory with the appropriate name, return its path
			if name == gitDirectoryName && info.IsDir() {
				return path.Join(cur, name), nil
			}
		}

		// if we failed, move up to the parent path
		cur = filepath.Dir(cur)
	}

	// if we've reached the root and haven't found a `.git` directory, return an
	// error.
	return "", fmt.Errorf("No Git directory found.")
}

// finds the current branch of the current Git repository
func gitCurrentBranch() (string, error) {
	gitPath, err := gitPath()
	if err != nil {
		return "", err
	}

	// this file contains a pointer to the current branch which we can parse to
	// determine the branch name.
	headPath := path.Join(gitPath, "HEAD")

	// read the HEAD file
	data, err := ioutil.ReadFile(headPath)
	if err != nil {
		return "", err
	}

	refSpec := string(data)

	// parse the HEAD file to get the branch name. the HEAD file contents look
	// something like: `ref: refs/heads/master`. we split into three parts
	refSpecParts := strings.SplitN(refSpec, "/", 3)
	if len(refSpecParts) != 3 {
		return "", fmt.Errorf("Could not parse HEAD file contents: '%s'", refSpec)
	}

	// return the third part of our split ref spec, the branch name
	return strings.TrimSpace(refSpecParts[2]), nil
}

func compressWithTruncator(s string, truncator rune, maxLen int) string {
	lenS := utf8.RuneCountInString(s)

	// if we're already short enough, bail
	if lenS <= maxLen {
		return s
	}

	// otherwise, calculate the reduction we need to fit into the max length
	reductionAmount := lenS - maxLen

	// remove the middle characters and replace them with our truncator
	middle := float64(lenS) / 2
	startIExact := middle - (float64(reductionAmount) / 2.0)
	endIExact := startIExact + float64(reductionAmount)
	startI := int(startIExact)
	endI := int(endIExact)

	// protect against overruns
	if startI < 0 {
		startI = 0
	}

	if endI >= lenS {
		endI = lenS
	}

	// construct a new string out of our old string's runes, replacing the
	// truncated ones with our truncator rune.
	truncatedS := make([]rune, 0, lenS-reductionAmount)
	truncated := false
	for i, ch := range s {
		if i < startI {
			truncatedS = append(truncatedS, ch)
		} else if !truncated {
			// add the truncator character if we haven't done so already
			truncatedS = append(truncatedS, truncator)
			truncated = true
		} else if i > endI {
			truncatedS = append(truncatedS, ch)
		}
	}

	return string(truncatedS)
}

// shortens and prettifies the given path, keeping it at or under the target
// length in runes.
func prettifyPath(p string, targetLength int) (string, error) {
	// clean up the path first
	p, err := filepath.Abs(filepath.Clean(p))
	if err != nil {
		return "", err
	}

	// if this path is in the current HOME directory, replace that dir with `~`
	homePath := os.Getenv("HOME")
	const homeTruncator = "~"
	if homePath != "" && strings.HasPrefix(p, homePath) {
		// mark that we're in the home directory for later
		p = homeTruncator + p[len(homePath):]
	}

	// save an original copy in case we can't do smart truncation well enough
	origP := p

	// determine how much we need to shorten our path to get it under the target,
	// i.e. how many characters of space we need to regain.
	neededGain := utf8.RuneCountInString(p) - targetLength

	// ALGORITHM:
	// truncate parent directories
	// * skips any leading home directory marker
	// * skips the base directory
	// * minimally truncates paths in order from longest to shortest

	const pathSeparator = string(os.PathSeparator)
	const segmentTruncator = '…'
	segments := strings.Split(p, pathSeparator)

	// inclusive/exclusive start/end indexes for the segments we'll try to
	// truncate in this pass.
	segmentsStartI := 0
	segmentsEndI := len(segments) - 1

	// truncate path segments by the minimum possible amount to try to reduce the
	// size of the overall path string.
	for i := segmentsStartI; i < segmentsEndI && neededGain > 0; i++ {
		// find the index of the longest remaining segment. linear search should be
		// fast enough for us since we'll probably never have more than 20 paths (on
		// a typical system at least, no?).
		longestI := segmentsStartI
		for j := segmentsStartI; j < segmentsEndI; j++ {
			// mark this as the longest segment if that's the case
			if len(segments[j]) > len(segments[longestI]) {
				longestI = j
			}
		}

		// operate on the longest segment
		segment := segments[longestI]
		lenSegment := utf8.RuneCountInString(segment)

		// calculate how much we can possibly gain from this segment, omitting the
		// start/end runes and one for the segment truncator.
		maxGain := lenSegment - 3

		// if we can reduce this segment...
		if maxGain > 0 {
			// reduce the segment by the smaller of the needed gain and the most we
			// can gain from this segment.
			reductionAmount := neededGain
			if reductionAmount > maxGain {
				reductionAmount = maxGain
			}

			// replace this segment with its truncated version
			segments[longestI] = compressWithTruncator(
				segment,
				segmentTruncator,
				lenSegment-reductionAmount,
			)

			// reduce the needed gain by the amount we just reduced our segment by
			neededGain -= reductionAmount
		}
	}

	// ALGORITHM:
	// * compress paths of length 3 to the first character and a truncator
	for i := segmentsStartI; i < segmentsEndI && neededGain > 0; i++ {
		segment := segments[i]
		lenSegment := utf8.RuneCountInString(segment)

		// if this segment is small enough, truncate to the first character and a
		// single truncator, saving a single character overall.
		if lenSegment == 3 {
			truncatedSegment := make([]rune, 0, 2)

			// append the first character, followed by a single truncator, then end.
			// this is a ghetto hack to easily pull out the first rune.
			for _, ch := range segment {
				truncatedSegment = append(truncatedSegment, ch, segmentTruncator)
				break
			}

			segments[i] = string(truncatedSegment)

			// reduce the needed gain by the amount we just reduced our segment by
			neededGain -= 1
		}
	}

	// ALGORITHM:
	// * compress already-compressed paths to a single character
	for i := segmentsStartI; i < segmentsEndI && neededGain > 0; i++ {
		segment := segments[i]
		lenSegment := utf8.RuneCountInString(segment)

		// if this segment is small enough and has already been truncated, truncate
		// to the first character alone.
		if lenSegment == 2 {
			lastRune, size := utf8.DecodeLastRuneInString(segment)
			if size > 0 && lastRune == segmentTruncator {
				for _, ch := range segment {
					segments[i] = string(ch)
					break
				}

				// reduce the needed gain by the single character
				neededGain -= 1
			}
		}
	}

	// ALGORITHM:
	// * if we're still out of space, just truncate the original path with a
	//   single truncator character.
	if neededGain > 0 {
		// compress the path by just truncating the original since we've lost so
		// much fidelity at this point it looks nicer this way. otherwise, the
		// result can become littered with random truncators.
		p = compressWithTruncator(origP, segmentTruncator, targetLength)
	} else {
		// put the path back together now that we're done modifying it by segment
		p = path.Join(segments...)
	}

	return p, nil
}

func main() {
	cwd, _ := os.Getwd()
	p, _ := prettifyPath(cwd, 20)
	fmt.Println("pretty path:", p)
}
