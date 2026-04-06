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

package protovalidate

import (
	"context"
	"errors"
	"fmt"
	"sync"

	bufprotovalidate "buf.build/go/protovalidate"
	"github.com/codesjoy/yggdrasil/v2/interceptor"
	ystatus "github.com/codesjoy/yggdrasil/v2/status"
	"github.com/codesjoy/yggdrasil/v2/stream"
	"google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/protobuf/proto"
)

// Validator validates protobuf messages.
type Validator interface {
	Validate(proto.Message, ...bufprotovalidate.ValidationOption) error
}

const defaultConfigName = "default"

var (
	defaultValidatorOnce sync.Once
	defaultValidator     Validator
	defaultValidatorErr  error
)

// New creates a reusable Protovalidate validator.
func New(options ...bufprotovalidate.ValidatorOption) (Validator, error) {
	return bufprotovalidate.New(options...)
}

// Validate validates a protobuf message with the shared default validator.
func Validate(msg proto.Message, options ...bufprotovalidate.ValidationOption) error {
	return bufprotovalidate.Validate(msg, options...)
}

// UnaryServerInterceptor validates unary inbound protobuf requests.
func UnaryServerInterceptor(validator Validator) interceptor.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		_ *interceptor.UnaryServerInfo,
		handler interceptor.UnaryHandler,
	) (any, error) {
		resolved, err := resolveValidator(validator)
		if err != nil {
			return nil, internalStatusError("initialize protovalidate validator", err)
		}
		if err := validateRequest(resolved, req); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

// StreamServerInterceptor validates each inbound message received from a stream.
func StreamServerInterceptor(validator Validator) interceptor.StreamServerInterceptor {
	return func(
		srv interface{},
		ss stream.ServerStream,
		_ *interceptor.StreamServerInfo,
		handler stream.Handler,
	) error {
		resolved, err := resolveValidator(validator)
		if err != nil {
			return internalStatusError("initialize protovalidate validator", err)
		}
		return handler(srv, &validatingServerStream{
			ServerStream: ss,
			validator:    resolved,
		})
	}
}

type validatingServerStream struct {
	stream.ServerStream
	validator Validator
}

func (s *validatingServerStream) RecvMsg(msg any) error {
	if err := s.ServerStream.RecvMsg(msg); err != nil {
		return err
	}
	return validateRequest(s.validator, msg)
}

func resolveValidator(validator Validator) (Validator, error) {
	if validator != nil {
		return validator, nil
	}

	defaultValidatorOnce.Do(func() {
		defaultValidator, defaultValidatorErr = New(defaultValidatorOptions(LoadConfig(defaultConfigName))...)
	})

	return defaultValidator, defaultValidatorErr
}

func defaultValidatorOptions(cfg Config) []bufprotovalidate.ValidatorOption {
	opts := make([]bufprotovalidate.ValidatorOption, 0, 1)
	if cfg.FailFast {
		opts = append(opts, bufprotovalidate.WithFailFast())
	}
	return opts
}

func validateRequest(validator Validator, msg any) error {
	if msg == nil {
		return nil
	}

	protoMsg, ok := msg.(proto.Message)
	if !ok {
		return nil
	}

	if err := validator.Validate(protoMsg); err != nil {
		return invalidArgumentStatus(err)
	}
	return nil
}

func invalidArgumentStatus(err error) error {
	st := ystatus.FromErrorCode(err, code.Code_INVALID_ARGUMENT)

	var validationErr *bufprotovalidate.ValidationError
	if errors.As(err, &validationErr) {
		return st.WithDetails(validationErr.ToProto())
	}

	return st
}

func internalStatusError(action string, err error) error {
	return ystatus.FromErrorCode(fmt.Errorf("%s: %w", action, err), code.Code_INTERNAL)
}
