//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package injectexec

import (
	"fmt"
	"regexp"
	"time"

	rex "github.com/heketi/heketi/pkg/remoteexec"
)

// Reaction represents the type of result or error to produce when
// a hook is encountered. This can be used to give back a fake
// result, an error, panic the server or pause for a time.
type Reaction struct {
	// Result is used to return a new "fake" result string
	Result string
	// Err is used to create a new forced error
	Err string
	// Pause will sleep for the given time in seconds. Pause can
	// be combined with the other other options.
	Pause uint64
	// Panic will trigger a panic.
	Panic string
}

// React triggers the kind of error or result the Reaction
// is configured for. The result and error returns are mutually
// exclusive.
func (r Reaction) React() (string, error) {
	if r.Pause != 0 {
		time.Sleep(time.Second * time.Duration(r.Pause))
	}
	if r.Panic != "" {
		panic(r.Panic)
	}
	if r.Err != "" {
		return "", fmt.Errorf(r.Err)
	}
	return r.Result, nil
}

// CmdHook is a hook for a given command. Provide a regex as a string
// to match a command flowing through one of the executors and if
// the hook matches, the hook's reaction will be called instead of
// the real command.
type CmdHook struct {
	Cmd      string
	Reaction Reaction
}

// Match returns true if the provided command matches the regex of
// the hook.
func (c *CmdHook) Match(command string) bool {
	m, e := regexp.MatchString(c.Cmd, command)
	return (e == nil && m)
}

// String returns a string representation of the hook.
func (c *CmdHook) String() string {
	return fmt.Sprintf("CmdHook(%v)", c.Cmd)
}

// ResultHook is a hook for a given command and result. This hook
// is checked after a command has run and only fires if both the
// command and the result match.
type ResultHook struct {
	Result string
	CmdHook
}

// Match returns true if the provided command and the command's result
// string match the hook's regexes for the Result and Cmd fields.
func (r *ResultHook) Match(command, result string) bool {
	m1, e1 := regexp.MatchString(r.Cmd, command)
	if e1 != nil {
		logger.Warning("regexp error: %v", e1)
	}
	m2, e2 := regexp.MatchString(r.Result, result)
	if e2 != nil {
		logger.Warning("regexp error: %v", e2)
	}
	return (e1 == nil && m1) && (e2 == nil && m2)
}

// String returns a string representation of the hook.
func (r *ResultHook) String() string {
	return fmt.Sprintf("ResultHook(%v)", r.Cmd)
}

type CmdHooks []CmdHook

type ResultHooks []ResultHook

// HookCommands checks a list of command hooks against a given command
// string. For the first matching hook the hook's reaction is returned.
func HookCommands(hooks CmdHooks, c string) rex.Result {

	logger.Info("Checking for hook on %v", c)
	for _, h := range hooks {
		if h.Match(c) {
			logger.Debug("found hook for %v: %v", c, h)
			hr, herr := h.Reaction.React()
			if herr != nil {
				return rex.Result{
					Completed: true,
					Err:       herr,
				}
			}
			return rex.Result{
				Completed: true,
				Output:    hr,
			}
		}
	}
	return rex.Result{}
}

// HookResults checks a list of result hooks against a given command
// and its result or error (as a string). For the first matching hook
// the hook's reaction is returned.
func HookResults(hooks ResultHooks, c string, result rex.Result) rex.Result {

	compare := result.Output
	if !result.Ok() {
		compare = result.Err.Error()
	}

	for _, h := range hooks {
		logger.Info("Checking for hook on %v -> %v", c, compare)
		if h.Match(c, compare) {
			logger.Debug("found hook for %v/%v: %v", c, compare, h)
			hr, herr := h.Reaction.React()
			if herr != nil {
				return rex.Result{
					Completed: true,
					Err:       herr,
				}
			}
			return rex.Result{
				Completed: true,
				Output:    hr,
			}
		}
	}
	return result
}
