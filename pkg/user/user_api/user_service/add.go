package user_service

import (
	"github.com/evgeniums/go-backend-helpers/pkg/api/api_server"
	"github.com/evgeniums/go-backend-helpers/pkg/user"
	"github.com/evgeniums/go-backend-helpers/pkg/user/user_api"
)

type AddEndpoint[U user.User] struct {
	api_server.EndpointBase
	UserEndpoint[U]
	setterBuilder func() user.UserFieldsSetter[U]
}

func (e *AddEndpoint[U]) HandleRequest(request api_server.Request) error {

	c := request.TraceInMethod("users.Add")
	defer request.TraceOutMethod()

	cmd := e.setterBuilder()
	err := request.ParseVerify(cmd)
	if err != nil {
		return err
	}

	resp := &user_api.UserResponse[U]{}
	resp.User, err = e.service.Users.Add(request, cmd.Login(), cmd.Password(), cmd.SetUserFields)
	if err != nil {
		return c.SetError(err)
	}

	request.Response().SetMessage(resp)

	return nil
}

func Add[U user.User](service *UserService[U], setterBuilder func() user.UserFieldsSetter[U]) *AddEndpoint[U] {
	e := &AddEndpoint[U]{}
	e.service = service
	e.setterBuilder = setterBuilder
	e.Construct(user_api.Add())
	return e
}
