//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package errtag

import "github.com/pkg/errors"

type errData struct {
	desc string
}

// ErrTag can create identifiable errors
type ErrTag struct {
	data *errData
}

// NewTag returns a new ErrTag
func NewTag(desc string) ErrTag {
	return ErrTag{&errData{desc}}
}

// In returns true if the error has been created from this ErrTag
func (e ErrTag) In(err error) bool {
	err = errors.Cause(err)
	if e2, ok := err.(errInstance); ok {
		return e.data == e2.data
	}
	return false
}

// Err creates an error from ErrTag
func (e ErrTag) Err() error {
	return errors.WithStack(errInstance(e))
}

type errInstance ErrTag

func (e errInstance) Error() string {
	return e.data.desc
}
