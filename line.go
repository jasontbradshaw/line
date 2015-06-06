package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
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

func main() {
	b, _ := gitCurrentBranch()
	fmt.Println(b)
}
