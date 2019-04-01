//
// Copyright (c) 2019 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package glusterfs

import (
	"fmt"
	"strings"
)

type MultiHostError struct {
	prefix string
	errors map[string]error
}

// NewMultiHostError returns a MultiHostError with the given
// prefix text. Prefix text will be used in the error string if
// more than one error is captured.
func NewMultiHostError(p string) *MultiHostError {
	return &MultiHostError{
		prefix: p,
		errors: map[string]error{},
	}
}

// Add an error originating with host `c` to the captured
// errors map.
func (m *MultiHostError) Add(c string, e error) {
	m.errors[c] = e
}

// Return the length of the captured errors map.
func (m *MultiHostError) Len() int {
	return len(m.errors)
}

// Shorten returns a simplified version of the errors that
// the MultiHostError may have captured. It returns nil if
// no errors were captured. It returns itself if more than one
// error was captured. It returns the original error if only
// one error was captured.
func (m *MultiHostError) Shorten() error {
	switch len(m.errors) {
	case 0:
		return nil
	case 1:
		for _, err := range m.errors {
			return err
		}
	}
	return m
}

// Error returns the error string for the multi host error.
// If only one error was captured, it returns the text of that
// error alone. If more than one error was captured, it returns
// formatted text containing all captured errors.
func (m *MultiHostError) Error() string {
	if len(m.errors) == 0 {
		return "(missing host error)"
	}
	if len(m.errors) == 1 {
		for _, v := range m.errors {
			return v.Error()
		}
	}
	errs := []string{}
	if m.prefix != "" {
		errs = append(errs, m.prefix)
	}
	ex := map[string]int{}
	for k, v := range m.errors {
		evalue := v.Error()
		ex[evalue] += 1
		errs = append(errs, fmt.Sprintf("Host %v: %v", k, evalue))
	}
	if len(ex) == 1 {
		// all the hosts returned the same error. return a simplified
		// error case.
		var evalue string
		for k := range ex {
			evalue = k
			break
		}
		return fmt.Sprintf("All %v hosts: %v", len(m.errors), evalue)
	}
	return strings.Join(errs, "\n")
}
