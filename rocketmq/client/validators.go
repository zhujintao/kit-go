/*
Licensed to the Apache Software Foundation (ASF) under one or more
contributor license agreements.  See the NOTICE file distributed with
this work for additional information regarding copyright ownership.
The ASF licenses this file to You under the Apache License, Version 2.0
(the "License"); you may not use this file except in compliance with
the License.  You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package client

import (
	"errors"
	"fmt"
	"regexp"
)

const (
	_ValidPattern       = "^[%|a-zA-Z0-9_-]+$"
	_CharacterMaxLength = 255
)

var (
	_Pattern = regexp.MustCompile(_ValidPattern)
)

func ValidateGroup(group string) error {
	if group == "" {
		return errors.New("consumerGroup is empty")
	}
	if len(group) > _CharacterMaxLength {
		return errors.New("the specified group is longer than group max length 255")
	}
	if !_Pattern.MatchString(group) {
		return fmt.Errorf("the specified group[%s] contains illegal characters, allowing only %s", group, _ValidPattern)
	}
	return nil
}
