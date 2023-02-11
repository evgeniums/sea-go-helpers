package user_service

import (
	"github.com/evgeniums/go-backend-helpers/pkg/api/api_server"
	"github.com/evgeniums/go-backend-helpers/pkg/user"
	"github.com/evgeniums/go-backend-helpers/pkg/user/user_api"
)

type SetPasswordEndpoint struct {
	SetUserFieldEndpoint
}

func (s *SetPasswordEndpoint) HandleRequest(request api_server.Request) error {

	c := request.TraceInMethod("users.SetPassword")
	defer request.TraceOutMethod()

	cmd := &user.UserPlainPassword{}
	err := request.ParseVerify(cmd)
	if err != nil {
		return err
	}

	err = s.users.SetPassword(request, request.GetResourceId(s.userTypeName), cmd.PlainPassword)
	if err != nil {
		return c.SetError(err)
	}

	return nil
}

func SetPassword(userTypeName string, users user.MainFieldSetters) api_server.ResourceEndpointI {
	e := &SetPasswordEndpoint{}
	return e.Init(e, userTypeName, "password", users, user_api.SetPassword())
}