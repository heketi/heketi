//
// Copyright (c) 2015 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package utils

import (
	"bufio"
	"os"
	"strings"

	"github.com/lpabon/godbc"
)

type Journal struct {
	Path  string
	Clean bool
}

// Create a new journal
func NewJournal(path string) *Journal {

	godbc.Check(path != "", path)

	j := &Journal{}

	// open output file
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}
	j.Path = path

	// close fo on exit and check for its returned error
	defer func() {
		if err := f.Close(); err != nil {
			panic(err)
		}
	}()

	return j
}

func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func (j *Journal) JournalCheck() (bool, error) {
	check := j.Clean
	path := j.Path
	scount := 0
	ecount := 0

	journal, err := readLines(path)
	if err != nil {
		check = false
	}

	for _, lines := range journal {
		if strings.Contains(lines, "START") {
			scount++
		}
		if strings.Contains(lines, "END") {
			ecount++
		}
	}

	if scount == ecount {
		check = true
	} else {
		check = false
	}
	j.Clean = check
	return check, nil
}

func (j *Journal) WriteJ(text string) error {
	path := j.Path
	f, err := os.OpenFile(path, os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	_, err = f.WriteString(text)
	if err != nil {
	}
	return err
}
