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
	"io"
	"log/slog"
	"os"
	"sync/atomic"

	"github.com/codesjoy/yggdrasil-ecosystem/modules/protovalidate/v3"
	userv1 "github.com/codesjoy/yggdrasil-ecosystem/modules/protovalidate/v3/examples/protogen/user/v1"
	"github.com/codesjoy/yggdrasil/v3"
)

const appName = "github.com.codesjoy.yggdrasil-ecosystem.modules.protovalidate.examples.quickstart"

type userService struct {
	userv1.UnimplementedUserServiceServer
	nextID atomic.Int64
}

func main() {
	err := yggdrasil.Run(
		context.Background(),
		appName,
		compose,
		yggdrasil.WithConfigPath("config.yaml"),
		protovalidate.WithModule(),
	)
	if err != nil {
		slog.Error("run protovalidate quickstart server", slog.Any("error", err))
		os.Exit(1)
	}
}

func compose(rt yggdrasil.Runtime) (*yggdrasil.BusinessBundle, error) {
	rt.Logger().Info("compose protovalidate quickstart bundle")

	return &yggdrasil.BusinessBundle{
		RPCBindings: []yggdrasil.RPCBinding{{
			ServiceName: userv1.UserServiceServiceDesc.ServiceName,
			Desc:        &userv1.UserServiceServiceDesc,
			Impl:        &userService{},
		}},
		Diagnostics: []yggdrasil.BundleDiag{{
			Code:    "protovalidate.quickstart",
			Message: "user service validation interceptors enabled",
		}},
	}, nil
}

func (s *userService) CreateUser(
	_ context.Context,
	req *userv1.CreateUserRequest,
) (*userv1.CreateUserResponse, error) {
	id := s.nextID.Add(1)
	slog.Info("create user handler reached", slog.String("email", req.GetEmail()))

	return &userv1.CreateUserResponse{
		Id:          fmt.Sprintf("user-%d", id),
		Email:       req.GetEmail(),
		DisplayName: req.GetDisplayName(),
	}, nil
}

func (s *userService) ImportUsers(stream userv1.UserServiceImportUsersServer) error {
	var imported int32
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&userv1.ImportUsersResponse{Imported: imported})
		}
		if err != nil {
			return err
		}

		imported++
		slog.Info("import user handler reached", slog.String("email", req.GetEmail()))
	}
}
