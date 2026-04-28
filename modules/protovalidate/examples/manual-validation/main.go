// Copyright 2022 The codesjoy Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	bufprotovalidate "buf.build/go/protovalidate"
	"github.com/codesjoy/yggdrasil-ecosystem/modules/protovalidate/v3"
	userv1 "github.com/codesjoy/yggdrasil-ecosystem/modules/protovalidate/v3/examples/protogen/user/v1"
)

func main() {
	validator, err := protovalidate.New()
	if err != nil {
		slog.Error("create validator", slog.Any("error", err))
		os.Exit(1)
	}

	valid := &userv1.CreateUserRequest{
		Email:       "ada@example.com",
		DisplayName: "Ada Lovelace",
		Age:         36,
	}
	if err := validator.Validate(valid); err != nil {
		slog.Error("valid request failed validation", slog.Any("error", err))
		os.Exit(1)
	}
	fmt.Println("valid request passed")

	invalid := &userv1.CreateUserRequest{
		Email:       "not-an-email",
		DisplayName: "A",
		Age:         8,
	}
	if err := protovalidate.Validate(invalid); err != nil {
		printViolations(err)
		return
	}

	slog.Error("invalid request unexpectedly passed validation")
	os.Exit(1)
}

func printViolations(err error) {
	var validationErr *bufprotovalidate.ValidationError
	if !errors.As(err, &validationErr) {
		fmt.Printf("validation failed: %v\n", err)
		return
	}

	fmt.Println("invalid request violations:")
	for _, violation := range validationErr.ToProto().GetViolations() {
		fmt.Printf(
			"- %s: %s\n",
			bufprotovalidate.FieldPathString(violation.GetField()),
			violation.GetMessage(),
		)
	}
}
