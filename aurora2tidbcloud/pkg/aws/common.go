// Copyright 2020 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package common

import (
	"errors"
	"time"
)

func WaitUntilResouceAvailable(_interval, _timeout time.Duration, expectNum int, _readResource func() (bool, error)) error {
	if _interval == 0 {
		_interval = 60 * time.Second
	}

	if _timeout == 0 {
		_timeout = 60 * time.Minute
	}

	timeout := time.After(_timeout)
	d := time.NewTicker(_interval)

	for {
		// Select statement
		select {
		case <-timeout:
			return errors.New("Timed out")
		case _ = <-d.C:
			isFinished, err := _readResource()
			if err != nil {
				return err
			}

			if isFinished == true {
				return nil
			}
		}
	}
}
