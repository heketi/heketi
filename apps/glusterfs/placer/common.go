//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package placer

import (
	"errors"

	"github.com/heketi/heketi/pkg/utils"
)

var (
	ErrNoDevices = errors.New("No devices available for brick placement.")
	errNotFound  = errors.New("Unknown ID")

	// define a logging object for the placer package
	// TODO: clean this up for more librarification
	logger = utils.NewLogger("[heketi]", utils.LEVEL_INFO)

	// for compat. with previous code
	KB = uint64(1)
)
