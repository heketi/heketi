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

package commands

import (
	"flag"
	"io"
	"os"
)

//make stdout "global" to command package
var (
	stdout io.Writer = os.Stdout
)

type Command interface {
	Name() string
	Exec([]string) error
}

type Commands []Command

type Cmd struct {
	name  string
	flags *flag.FlagSet
}
