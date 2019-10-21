// Copyright 2019 Cloudbase Solutions SRL
//
//    Licensed under the Apache License, Version 2.0 (the "License"); you may
//    not use this file except in compliance with the License. You may obtain
//    a copy of the License at
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//    WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//    License for the specific language governing permissions and limitations
//    under the License.

package stdout

import (
	"fmt"

	"coriolis-logger/logging"
)

func NewStdOutWriter() (logging.Writer, error) {
	return &StdOutWriter{}, nil
}

var _ logging.Writer = (*StdOutWriter)(nil)

// StdOutWriter is a simple writer that writes to stdout
type StdOutWriter struct{}

func (i *StdOutWriter) Write(logMsg logging.LogMessage) error {
	fmt.Println(logMsg.Message)
	return nil
}
