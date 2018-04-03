//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package cmds

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func setTagsCommand(cmd *cobra.Command,
	fetchTags func(id string) (map[string]string, error),
	submitTags func(id string, tags map[string]string) error) error {

	//ensure proper number of args
	s := cmd.Flags().Args()
	if len(s) < 1 {
		return errors.New("id required")
	}
	if len(s) < 2 {
		return errors.New("at least one tag:value pair expected")
	}

	exact, err := cmd.Flags().GetBool("exact")
	if err != nil {
		return err
	}

	id = s[0]

	newTags := map[string]string{}
	for _, t := range s[1:] {
		parts := strings.SplitN(t, ":", 2)
		if len(parts) < 2 {
			return fmt.Errorf(
				"expected colon (:) between tag name and value, got: %v",
				t)
		}
		newTags[parts[0]] = parts[1]
	}

	var setTags map[string]string
	if exact {
		setTags = newTags
	} else {
		oldTags, err := fetchTags(id)
		if err != nil {
			return err
		}
		if oldTags == nil {
			setTags = map[string]string{}
		} else {
			setTags = oldTags
		}
		for k, v := range newTags {
			setTags[k] = v
		}
	}

	return submitTags(id, setTags)
}

func rmTagsCommand(cmd *cobra.Command,
	fetchTags func(id string) (map[string]string, error),
	submitTags func(id string, tags map[string]string) error) error {

	//ensure proper number of args
	s := cmd.Flags().Args()
	if len(s) < 1 {
		return errors.New("id required")
	}

	id := s[0]

	removeAll, err := cmd.Flags().GetBool("all")
	if err != nil {
		return err
	}

	if len(s) < 2 && !removeAll {
		return errors.New("at least one tag expected")
	}
	if len(s) >= 2 && removeAll {
		return errors.New("--all may not be combined with named tags")
	}

	var setTags map[string]string
	if removeAll {
		setTags = map[string]string{}
	} else {
		oldTags, err := fetchTags(id)
		if err != nil {
			return err
		}
		if oldTags == nil {
			setTags = map[string]string{}
		} else {
			setTags = oldTags
		}
		for _, k := range s[1:] {
			delete(setTags, k)
		}
	}

	return submitTags(id, setTags)
}
