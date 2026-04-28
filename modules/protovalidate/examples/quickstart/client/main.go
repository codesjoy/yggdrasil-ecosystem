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
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	userv1 "github.com/codesjoy/yggdrasil-ecosystem/modules/protovalidate/v3/examples/protogen/user/v1"
	yapp "github.com/codesjoy/yggdrasil/v3/app"
	ystatus "github.com/codesjoy/yggdrasil/v3/rpc/status"
	"google.golang.org/genproto/googleapis/rpc/code"
)

const serverAppName = "github.com.codesjoy.yggdrasil-ecosystem.modules.protovalidate.examples.quickstart"

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	app, err := yapp.New("protovalidate-quickstart-client", yapp.WithConfigPath("config.yaml"))
	if err != nil {
		slog.Error("create client app", slog.Any("error", err))
		os.Exit(1)
	}
	defer func() {
		if err := app.Stop(context.Background()); err != nil {
			slog.Error("stop client app", slog.Any("error", err))
		}
	}()

	cli, err := app.NewClient(ctx, serverAppName)
	if err != nil {
		slog.Error("create client", slog.Any("error", err))
		os.Exit(1)
	}
	defer func() { _ = cli.Close() }()

	client := userv1.NewUserServiceClient(cli)
	if err := run(ctx, client); err != nil {
		slog.Error("run quickstart client", slog.Any("error", err))
		os.Exit(1)
	}
}

func run(ctx context.Context, client userv1.UserServiceClient) error {
	resp, err := client.CreateUser(ctx, &userv1.CreateUserRequest{
		Email:       "ada@example.com",
		DisplayName: "Ada Lovelace",
		Age:         36,
	})
	if err != nil {
		return fmt.Errorf("valid CreateUser: %w", err)
	}
	fmt.Printf("valid unary accepted: %s %s\n", resp.GetId(), resp.GetEmail())

	_, err = client.CreateUser(ctx, &userv1.CreateUserRequest{
		Email:       "not-an-email",
		DisplayName: "A",
		Age:         8,
	})
	if err := expectInvalidArgument("invalid unary rejected", err); err != nil {
		return err
	}

	stream, err := client.ImportUsers(ctx)
	if err != nil {
		return fmt.Errorf("open ImportUsers stream: %w", err)
	}
	if err := stream.Send(&userv1.CreateUserRequest{
		Email:       "grace@example.com",
		DisplayName: "Grace Hopper",
		Age:         85,
	}); err != nil {
		return fmt.Errorf("send valid stream request: %w", err)
	}
	if err := stream.Send(&userv1.CreateUserRequest{
		Email:       "bad-email",
		DisplayName: "B",
		Age:         7,
	}); err != nil {
		if err := expectInvalidArgument("invalid stream send rejected", err); err != nil {
			return err
		}
		return nil
	}
	_, err = stream.CloseAndRecv()
	return expectInvalidArgument("invalid stream close rejected", err)
}

func expectInvalidArgument(label string, err error) error {
	if err == nil {
		return fmt.Errorf("%s: expected INVALID_ARGUMENT, got nil", label)
	}

	st, ok := ystatus.CoverError(err)
	if !ok {
		return fmt.Errorf("%s: expected Yggdrasil status, got %T: %w", label, err, err)
	}
	if st.Code() != code.Code_INVALID_ARGUMENT {
		return fmt.Errorf("%s: expected INVALID_ARGUMENT, got %s: %w", label, st.Code(), err)
	}

	fmt.Printf("%s: %s\n", label, st.Code())
	return nil
}
