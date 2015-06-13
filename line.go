package main

import (
	"crypto/md5"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"
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
func gitCurrentBranch() string {
	gitPath, err := gitPath()
	if err != nil {
		return ""
	}

	// this file contains a pointer to the current branch which we can parse to
	// determine the branch name.
	headPath := path.Join(gitPath, "HEAD")

	// read the HEAD file
	data, err := ioutil.ReadFile(headPath)
	if err != nil {
		return ""
	}

	refSpec := strings.TrimSpace(string(data))

	// parse the HEAD file to get the branch name. the HEAD file contents look
	// something like: `ref: refs/heads/master`. we split into three parts, then
	// use whatever's left over as the branch name. If it doesn't split, it's
	// probably a commit hash, in which case we use the first 8 characters of it
	// as the branch name.
	refSpecParts := strings.SplitN(refSpec, "/", 3)
	branchName := ""
	if len(refSpecParts) == 3 {
		// use the last part as the branch name
		branchName = strings.TrimSpace(refSpecParts[2])
	} else if len(refSpecParts) == 1 && len(refSpec) == 40 {
		// we got a commit hash, use the first 7 characters as the branch name
		branchName = refSpec[0:7]
	} else {
		// notify that we failed
		branchName = "BAD_REF_SPEC (" + refSpec + ")"
	}

	// return the third part of our split ref spec, the branch name
	return branchName
}

// gets the current status symbols for the existing git repository as a map of
// file name to status symbol, or nil if there's no repository.
func gitCurrentStatus() map[string]string {
	out, err := exec.Command("git", "status", "--porcelain").CombinedOutput()
	if err != nil {
		return nil
	}

	// turn the output into a map of file to status string
	files := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		// trim whitespace so we can reliably split out the status/name
		line = strings.TrimSpace(line)

		// split into a (status, file) pair
		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 2 {
			files[parts[1]] = parts[0]
		}
	}

	return files
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

// given a string, returns a hex color based on its contents
func colorHash(input string) int {
	// turn the user/host combination into a color, then use that color as the
	// foreground color of the `@` symbol, to help distinguish between terminals
	// running on different hosts.
	md5Hash := md5.New()
	io.WriteString(md5Hash, input)
	sum := md5Hash.Sum(nil)

	// use the first three bytes as an RGB color, then convert to HSL so we can
	// easily keep the color in a nice range. then convert back to RGB, then back
	// to hex so we can display it!
	r := int(sum[0])
	g := int(sum[1])
	b := int(sum[2])

	h, s, l := rgbToHSL(r, g, b)

	// scale our lightness to keep it readable against a dark background
	minLightness := 0.3
	maxLightness := 0.85
	l = (l * (maxLightness - minLightness)) + minLightness

	r, g, b = hslToRGB(h, s, l)
	return rgbToHex(r, g, b)
}

// returns the user/hostname of the system with a specifically-colored `@`
func userAndHost() string {
	// never mind the error, just use whatever came back
	host, _ := os.Hostname()
	user := os.Getenv("USER")

	c := colorHash(user + host)

	return trueColored("[", c) + user + trueColored("@", c) + host + trueColored("]", c)
}

func currentTime() string {
	return fmt.Sprintf("%d", time.Now().Unix())
}

// print the status line!
func main() {
	cwd, _ := os.Getwd()
	prettyPath, _ := prettifyPath(cwd, 60)
	branch := gitCurrentBranch()

	// pick a color for the branch depending on status output
	branchColor := COLOR_GREEN
	statuses := gitCurrentStatus()
	if statuses != nil && len(statuses) > 0 {
		hasUntracked := false
		hasModified := false

		for _, status := range statuses {
			// true if we have untracked or added files
			hasUntracked = hasUntracked || strings.ContainsAny(status, "A?")

			// true if we have modified, renamed, deleted, or unstaged files
			hasModified = hasModified || strings.ContainsAny(status, "MRDU")
		}

		if hasUntracked && !hasModified {
			branchColor = COLOR_YELLOW
		} else if hasModified {
			branchColor = COLOR_RED
		}
	}

	fmt.Printf("┌╼ %s %s %s %s\n└╼ \n",
		colored(currentTime(), COLOR_MAGENTA),
		userAndHost(),
		colored(prettyPath, COLOR_BLUE),
		colored(branch, branchColor))
}
