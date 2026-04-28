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
	"io"
	"testing"

	validatepb "buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
	bufprotovalidate "buf.build/go/protovalidate"
	"github.com/codesjoy/yggdrasil/v3"
	"github.com/codesjoy/yggdrasil/v3/capabilities"
	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/rpc/interceptor"
	"github.com/codesjoy/yggdrasil/v3/rpc/metadata"
	ystatus "github.com/codesjoy/yggdrasil/v3/rpc/status"
	ystream "github.com/codesjoy/yggdrasil/v3/rpc/stream"
	"google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

func TestValidate(t *testing.T) {
	desc := emailRequestDescriptor(t)

	valid := newEmailRequest(t, desc, "user@example.com")
	if err := Validate(valid); err != nil {
		t.Fatalf("Validate(valid) error = %v", err)
	}

	invalid := newEmailRequest(t, desc, "not-an-email")
	err := Validate(invalid)
	if err == nil {
		t.Fatal("Validate(invalid) error = nil, want validation error")
	}

	var validationErr *bufprotovalidate.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("Validate(invalid) error = %T, want *protovalidate.ValidationError", err)
	}
	if got := len(validationErr.ToProto().GetViolations()); got == 0 {
		t.Fatal("Validate(invalid) returned no violations")
	}
}

func TestNew(t *testing.T) {
	validator, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := validator.Validate(newEmailRequest(t, emailRequestDescriptor(t), "user@example.com")); err != nil {
		t.Fatalf("validator.Validate(valid) error = %v", err)
	}
}

func TestModuleConfig(t *testing.T) {
	mod, ok := Module().(*protovalidateModule)
	if !ok {
		t.Fatalf("Module() type = %T, want *protovalidateModule", Module())
	}

	if mod.Name() != capabilityName {
		t.Fatalf("Name() = %q, want %q", mod.Name(), capabilityName)
	}
	if mod.ConfigPath() != "yggdrasil.protovalidate" {
		t.Fatalf("ConfigPath() = %q, want yggdrasil.protovalidate", mod.ConfigPath())
	}

	view := config.NewView("yggdrasil.protovalidate", config.NewSnapshot(map[string]any{
		"default": map[string]any{
			"failFast": true,
		},
	}))
	if err := mod.Init(context.Background(), view); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	if got := mod.defaultConfig(); !got.FailFast {
		t.Fatal("defaultConfig().FailFast = false, want true")
	}
}

func TestModuleUnaryServerInterceptorReadsConfig(t *testing.T) {
	desc := multiEmailRequestDescriptor(t)
	info := &interceptor.UnaryServerInfo{FullMethod: "/test.user.v1.UserService/Create"}
	req := newMultiEmailRequest(t, desc, "bad-primary", "bad-secondary")

	t.Run("config off accumulates all violations", func(t *testing.T) {
		unary := moduleUnaryServerInterceptor(t, Config{FailFast: false})

		handlerCalled := false
		_, err := unary(
			context.Background(),
			req,
			info,
			func(context.Context, any) (any, error) {
				handlerCalled = true
				return "ok", nil
			},
		)
		if err == nil {
			t.Fatal("default unary error = nil, want invalid argument")
		}
		if handlerCalled {
			t.Fatal("handler was called for invalid request")
		}

		st, ok := ystatus.CoverError(err)
		if !ok {
			t.Fatalf("CoverError(%T) ok = false", err)
		}
		if got := len(extractViolations(t, st).GetViolations()); got != 2 {
			t.Fatalf("violations len = %d, want 2", got)
		}
	})

	t.Run("config on fails fast", func(t *testing.T) {
		unary := moduleUnaryServerInterceptor(t, Config{FailFast: true})

		handlerCalled := false
		_, err := unary(
			context.Background(),
			req,
			info,
			func(context.Context, any) (any, error) {
				handlerCalled = true
				return "ok", nil
			},
		)
		if err == nil {
			t.Fatal("default unary error = nil, want invalid argument")
		}
		if handlerCalled {
			t.Fatal("handler was called for invalid request")
		}

		st, ok := ystatus.CoverError(err)
		if !ok {
			t.Fatalf("CoverError(%T) ok = false", err)
		}
		if got := len(extractViolations(t, st).GetViolations()); got != 1 {
			t.Fatalf("violations len = %d, want 1", got)
		}
	})
}

func TestModuleExposesV3Capabilities(t *testing.T) {
	mod, ok := Module().(*protovalidateModule)
	if !ok {
		t.Fatalf("Module() type = %T, want *protovalidateModule", Module())
	}

	caps := mod.Capabilities()
	want := map[string]bool{
		capabilities.UnaryServerInterceptorSpec.Name + "/" + capabilityName:  false,
		capabilities.StreamServerInterceptorSpec.Name + "/" + capabilityName: false,
	}
	for _, cap := range caps {
		key := cap.Spec.Name + "/" + cap.Name
		if _, ok := want[key]; ok {
			want[key] = true
		}
	}
	for key, seen := range want {
		if !seen {
			t.Fatalf("capability %s not exposed; got %#v", key, caps)
		}
	}
}

func TestWithModule(t *testing.T) {
	if WithModule() == nil {
		t.Fatal("WithModule() = nil")
	}
	app, err := yggdrasil.New("protovalidate-test", WithModule())
	if err != nil {
		t.Fatalf("yggdrasil.New() error = %v", err)
	}
	if app == nil {
		t.Fatal("yggdrasil.New() app = nil")
	}
}

func moduleUnaryServerInterceptor(
	t *testing.T,
	cfg Config,
) interceptor.UnaryServerInterceptor {
	t.Helper()

	mod, ok := Module().(*protovalidateModule)
	if !ok {
		t.Fatalf("Module() type = %T, want *protovalidateModule", Module())
	}
	view := config.NewView("yggdrasil.protovalidate", config.NewSnapshot(map[string]any{
		"default": map[string]any{
			"failFast": cfg.FailFast,
		},
	}))
	if err := mod.Init(context.Background(), view); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	for _, cap := range mod.Capabilities() {
		if cap.Spec.Name != capabilities.UnaryServerInterceptorSpec.Name ||
			cap.Name != capabilityName {
			continue
		}
		provider, ok := cap.Value.(interceptor.UnaryServerInterceptorProvider)
		if !ok {
			t.Fatalf("unary provider type = %T", cap.Value)
		}
		return provider.New()
	}

	t.Fatal("protovalidate unary server interceptor provider not found")
	return nil
}

func TestUnaryServerInterceptor(t *testing.T) {
	validator, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	unary := UnaryServerInterceptor(validator)
	info := &interceptor.UnaryServerInfo{FullMethod: "/test.user.v1.UserService/Create"}
	desc := emailRequestDescriptor(t)

	t.Run("valid proto request reaches handler", func(t *testing.T) {
		handlerCalled := false
		resp, err := unary(
			context.Background(),
			newEmailRequest(t, desc, "user@example.com"),
			info,
			func(context.Context, any) (any, error) {
				handlerCalled = true
				return "ok", nil
			},
		)
		if err != nil {
			t.Fatalf("unary(valid) error = %v", err)
		}
		if !handlerCalled {
			t.Fatal("handler was not called for valid request")
		}
		if resp != "ok" {
			t.Fatalf("unary(valid) response = %v, want ok", resp)
		}
	})

	t.Run("invalid proto request returns invalid argument with violations", func(t *testing.T) {
		handlerCalled := false
		_, err := unary(
			context.Background(),
			newEmailRequest(t, desc, "not-an-email"),
			info,
			func(context.Context, any) (any, error) {
				handlerCalled = true
				return "ok", nil
			},
		)
		if err == nil {
			t.Fatal("unary(invalid) error = nil, want invalid argument")
		}
		if handlerCalled {
			t.Fatal("handler was called for invalid request")
		}

		st, ok := ystatus.CoverError(err)
		if !ok {
			t.Fatalf("CoverError(%T) ok = false", err)
		}
		if st.Code() != code.Code_INVALID_ARGUMENT {
			t.Fatalf("status code = %v, want %v", st.Code(), code.Code_INVALID_ARGUMENT)
		}

		violations := extractViolations(t, st)
		if got := len(violations.GetViolations()); got != 1 {
			t.Fatalf("violations len = %d, want 1", got)
		}
		if got := bufprotovalidate.FieldPathString(violations.GetViolations()[0].GetField()); got != "email" {
			t.Fatalf("violation field path = %q, want email", got)
		}
	})

	t.Run("non proto request passes through unchanged", func(t *testing.T) {
		handlerCalled := false
		resp, err := unary(
			context.Background(),
			struct{ Email string }{Email: "not-an-email"},
			info,
			func(_ context.Context, req any) (any, error) {
				handlerCalled = true
				return req, nil
			},
		)
		if err != nil {
			t.Fatalf("unary(non-proto) error = %v", err)
		}
		if !handlerCalled {
			t.Fatal("handler was not called for non-proto request")
		}
		if _, ok := resp.(struct{ Email string }); !ok {
			t.Fatalf("unary(non-proto) response type = %T", resp)
		}
	})
}

func TestStreamServerInterceptor(t *testing.T) {
	validator, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	streamInt := StreamServerInterceptor(validator)
	info := &interceptor.StreamServerInfo{
		FullMethod:     "/test.user.v1.UserService/Chat",
		IsClientStream: true,
	}

	t.Run("valid inbound stream message reaches handler", func(t *testing.T) {
		desc := emailRequestDescriptor(t)
		handlerCalled := false
		err := streamInt(
			struct{}{},
			&mockServerStream{
				recv: recvDynamicMessage(t, newEmailRequest(t, desc, "user@example.com")),
			},
			info,
			func(_ interface{}, ss ystream.ServerStream) error {
				handlerCalled = true
				msg := dynamicpb.NewMessage(desc)
				if err := ss.RecvMsg(msg); err != nil {
					return err
				}
				if got := msg.Get(desc.Fields().ByName(protoreflect.Name("email"))).String(); got != "user@example.com" {
					return fmt.Errorf("received email = %q, want user@example.com", got)
				}
				return nil
			},
		)
		if err != nil {
			t.Fatalf("stream(valid) error = %v", err)
		}
		if !handlerCalled {
			t.Fatal("handler was not called for valid stream message")
		}
	})

	t.Run(
		"invalid inbound stream message fails before business logic continues",
		func(t *testing.T) {
			desc := emailRequestDescriptor(t)
			businessContinued := false
			err := streamInt(
				struct{}{},
				&mockServerStream{
					recv: recvDynamicMessage(t, newEmailRequest(t, desc, "not-an-email")),
				},
				info,
				func(_ interface{}, ss ystream.ServerStream) error {
					msg := dynamicpb.NewMessage(desc)
					if err := ss.RecvMsg(msg); err != nil {
						return err
					}
					businessContinued = true
					return nil
				},
			)
			if err == nil {
				t.Fatal("stream(invalid) error = nil, want invalid argument")
			}
			if businessContinued {
				t.Fatal("business logic continued after invalid stream message")
			}

			st, ok := ystatus.CoverError(err)
			if !ok {
				t.Fatalf("CoverError(%T) ok = false", err)
			}
			if st.Code() != code.Code_INVALID_ARGUMENT {
				t.Fatalf("status code = %v, want %v", st.Code(), code.Code_INVALID_ARGUMENT)
			}
			if got := len(extractViolations(t, st).GetViolations()); got != 1 {
				t.Fatalf("violations len = %d, want 1", got)
			}
		},
	)
}

func extractViolations(t *testing.T, st *ystatus.Status) *validatepb.Violations {
	t.Helper()

	details := st.Status().GetDetails()
	if len(details) == 0 {
		t.Fatal("status contains no details")
	}

	violations := &validatepb.Violations{}
	if err := details[0].UnmarshalTo(violations); err != nil {
		t.Fatalf("UnmarshalTo(Violations) error = %v", err)
	}
	return violations
}

func recvDynamicMessage(t *testing.T, src proto.Message) func(any) error {
	t.Helper()

	called := false
	return func(dst any) error {
		if called {
			return io.EOF
		}
		called = true

		dstProto, ok := dst.(proto.Message)
		if !ok {
			return fmt.Errorf("destination %T does not implement proto.Message", dst)
		}
		proto.Merge(dstProto, src)
		return nil
	}
}

func newEmailRequest(
	t *testing.T,
	desc protoreflect.MessageDescriptor,
	email string,
) proto.Message {
	t.Helper()

	msg := dynamicpb.NewMessage(desc)
	msg.Set(desc.Fields().ByName(protoreflect.Name("email")), protoreflect.ValueOfString(email))
	return msg
}

func newMultiEmailRequest(
	t *testing.T,
	desc protoreflect.MessageDescriptor,
	primary string,
	secondary string,
) proto.Message {
	t.Helper()

	msg := dynamicpb.NewMessage(desc)
	msg.Set(
		desc.Fields().ByName(protoreflect.Name("primary_email")),
		protoreflect.ValueOfString(primary),
	)
	msg.Set(
		desc.Fields().ByName(protoreflect.Name("secondary_email")),
		protoreflect.ValueOfString(secondary),
	)
	return msg
}

func emailRequestDescriptor(t *testing.T) protoreflect.MessageDescriptor {
	t.Helper()

	fileProto := &descriptorpb.FileDescriptorProto{
		Syntax:     proto.String("proto3"),
		Name:       proto.String("test/protovalidate/request.proto"),
		Package:    proto.String("test.protovalidate.v1"),
		Dependency: []string{"buf/validate/validate.proto"},
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("CreateUserRequest"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:     proto.String("email"),
						JsonName: proto.String("email"),
						Number:   proto.Int32(1),
						Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
						Options:  emailFieldOptions(),
					},
				},
			},
		},
	}

	fileDesc, err := protodesc.NewFile(fileProto, protoregistry.GlobalFiles)
	if err != nil {
		t.Fatalf("protodesc.NewFile() error = %v", err)
	}

	msgDesc := fileDesc.Messages().ByName(protoreflect.Name("CreateUserRequest"))
	if msgDesc == nil {
		t.Fatal("CreateUserRequest descriptor not found")
	}
	return msgDesc
}

func multiEmailRequestDescriptor(t *testing.T) protoreflect.MessageDescriptor {
	t.Helper()

	fileProto := &descriptorpb.FileDescriptorProto{
		Syntax:     proto.String("proto3"),
		Name:       proto.String("test/protovalidate/multi_request.proto"),
		Package:    proto.String("test.protovalidate.v1"),
		Dependency: []string{"buf/validate/validate.proto"},
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("CreateMultiEmailRequest"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:     proto.String("primary_email"),
						JsonName: proto.String("primaryEmail"),
						Number:   proto.Int32(1),
						Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
						Options:  emailFieldOptions(),
					},
					{
						Name:     proto.String("secondary_email"),
						JsonName: proto.String("secondaryEmail"),
						Number:   proto.Int32(2),
						Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
						Options:  emailFieldOptions(),
					},
				},
			},
		},
	}

	fileDesc, err := protodesc.NewFile(fileProto, protoregistry.GlobalFiles)
	if err != nil {
		t.Fatalf("protodesc.NewFile() error = %v", err)
	}

	msgDesc := fileDesc.Messages().ByName(protoreflect.Name("CreateMultiEmailRequest"))
	if msgDesc == nil {
		t.Fatal("CreateMultiEmailRequest descriptor not found")
	}
	return msgDesc
}

func emailFieldOptions() *descriptorpb.FieldOptions {
	fieldOptions := &descriptorpb.FieldOptions{}
	fieldRules := (validatepb.FieldRules_builder{
		String: (validatepb.StringRules_builder{
			Email: proto.Bool(true),
		}).Build(),
	}).Build()
	proto.SetExtension(fieldOptions, validatepb.E_Field, fieldRules)
	return fieldOptions
}

type mockServerStream struct {
	ctx  context.Context
	recv func(any) error
}

func (m *mockServerStream) Context() context.Context {
	if m.ctx != nil {
		return m.ctx
	}
	return context.Background()
}

func (m *mockServerStream) RecvMsg(msg any) error {
	if m.recv == nil {
		return io.EOF
	}
	return m.recv(msg)
}

func (*mockServerStream) SendMsg(any) error {
	return nil
}

func (*mockServerStream) SetHeader(metadata.MD) error {
	return nil
}

func (*mockServerStream) SendHeader(metadata.MD) error {
	return nil
}

func (*mockServerStream) SetTrailer(metadata.MD) {}
