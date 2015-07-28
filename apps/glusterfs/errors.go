//
// Copyright (c) 2015 The heketi Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package glusterfs

import (
	"errors"
	"net/http"
)

var (
	ErrNoSpace          = errors.New("No space")
	ErrNotFound         = errors.New("Id not found")
	ErrConflict         = errors.New(http.StatusText(http.StatusConflict))
	ErrMaxBricks        = errors.New("Maximum number of bricks reached.")
	ErrMininumBrickSize = errors.New("Minimum brick size limit reached.  Out of space.")
	ErrDbAccess         = errors.New("Unable to access db")
	ErrAccessList       = errors.New("Unable to access list")
)
