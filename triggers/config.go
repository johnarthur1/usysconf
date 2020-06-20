// Copyright © 2019-2020 Solus Project
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

package triggers

import (
	"fmt"
	"github.com/BurntSushi/toml"
	wlog "github.com/DataDrake/waterlog"
	"io/ioutil"
	"os"
	"path/filepath"
)

// Config contains all the details of the configuration file to be executed.
type Config struct {
	Description string            `toml:"description"`
	Bins        []Bin             `toml:"bins"`
	Skip        *Skip             `toml:"skip,omitempty"`
	Check       *Check            `toml:"check,omitempty"`
	Env         map[string]string `toml:"env"`
	RemoveDirs  *Remove           `toml:"remove,omitempty"`
}

// Load reads a Trigger configuration from a file and parses it
func (c *Config) Load(path string) error {
	// Check if this is a valid file path
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return err
	}

	// Read the configuration into the program
	cfg, err := ioutil.ReadFile(filepath.Clean(path))
	if err != nil {
		return fmt.Errorf("unable to read config file located at %s", path)
	}

	// Save the configuration into the content structure
	if err := toml.Unmarshal(cfg, c); err != nil {
		return fmt.Errorf("unable to read config file located at %s due to %s", path, err.Error())
	}
	return nil
}

// Validate checks for errors in a Trigger configuration
func (c *Config) Validate() error {
	// Verify that there is at least one binary to execute, otherwise there
	// is no need to continue
	if len(c.Bins) == 0 {
		return fmt.Errorf("triggers must contain at least one [[bin]]")
	}

	// Verify that the text to be outputted to the user does not exceed 42
	// characters to make the outputted lines uniform in length
	for _, bin := range c.Bins {
		if len(bin.Task) > 42 {
			return fmt.Errorf("the task: `%s` text cannot exceed the length of 42", bin.Task)
		}
	}
	return nil
}

// Execute runs a trigger based on its configuration and the applicable scope
func (c *Config) Execute(s Scope) []Output {
	outs := make([]Output, 0)
	if c.SkipProcessing(s) {
		outs = append(outs, Output{Status: Skipped})
		return outs
	}
	rm := c.RemoveDirs
	if rm != nil {
		if err := rm.Execute(s); err != nil {
			o := Output{
				Message: fmt.Sprintf("error removing path: %s\n", err.Error()),
				Status:  Failure,
			}
			outs = append(outs, o)
			return outs
		}
	}

	bins, outs := c.GetAllBins()

	var out Output
	for i, b := range bins {
		out = b.Execute(s, c.Env)
		outs[i].Status = out.Status
		outs[i].Message = out.Message
	}
	return outs
}

// SkipProcessing will process the skip and check elements of the configuration
// and see if it should not be executed.
func (c *Config) SkipProcessing(s Scope) bool {

	// Check if the paths exist, if not skip
	if c.Check != nil {
		if err := c.Check.ResolvePaths(); err != nil {
			wlog.Errorln(err.Error())
			return true
		}
	}

	// Even if the skip element exists, if the force flag is present,
	// continue processing
	if s.Forced {
		return false
	}

	if c.Skip == nil {
		return false
	}

	// If the skip element exists and the chroot flag is present, skip
	if c.Skip.Chroot && s.Chroot {
		return true
	}

	// If the skip element exists and the live flag is present, skip
	if c.Skip.Live && s.Live {
		return true
	}

	// Process through the skip paths, and if one is present within the
	// system, skip
	for _, p := range c.Skip.Paths {
		if _, err := os.Stat(filepath.Clean(p)); !os.IsNotExist(err) {
			return true
		}
	}

	return false
}

// GetAllBins Process through the binaries of the configuration and check if
// the "***" replace sequence exists in the arguments and create separate
// binaries to be executed.
func (c *Config) GetAllBins() (nbins []Bin, outputs []Output) {
	for _, b := range c.Bins {
		bs, outs := b.FanOut()
		nbins = append(nbins, bs...)
		outputs = append(outputs, outs...)
	}
	return
}