package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
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

func main() {
	g, _ := gitPath()
	fmt.Printf(".git path: %s\n", g)
}
