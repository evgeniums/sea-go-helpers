package user_service

import (
	"github.com/evgeniums/go-backend-helpers/pkg/api/api_server"
	"github.com/evgeniums/go-backend-helpers/pkg/user"
	"github.com/evgeniums/go-backend-helpers/pkg/user/user_api"
)

type UserEndpoint[U user.User] struct {
	service *UserService[U]
}

type UserService[U user.User] struct {
	api_server.ServiceBase
	Users        user.Users[U]
	UserTypeName string
}

func NewUserService[U user.User](userController user.Users[U],
	setterBuilder func() user.UserFieldsSetter[U],
	userTypeName ...string) *UserService[U] {

	userType, serviceName, users, _ := user_api.PrepareResources(userTypeName...)
	s := &UserService[U]{}
	s.Init(serviceName)
	s.UserTypeName = userType

	s.Users = userController
	s.AddChild(users)

	users.AddOperation(List(s))
	users.AddOperation(Add(s, setterBuilder))

	// user.AddOperation(Update(s))

	return s
}
