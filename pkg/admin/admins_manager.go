package admin

import (
	"github.com/evgeniums/go-backend-helpers/pkg/op_context"
	"github.com/evgeniums/go-backend-helpers/pkg/user"
)

type Manager struct {
	*user.UsersWithSessionBase[*Admin, *AdminSession, *AdminSessionClient]
}

type AdminControllers = user.UsersWithSessionBaseConfig[*Admin]

func NewManager(controllers ...AdminControllers) *Manager {
	m := &Manager{UsersWithSessionBase: user.NewUsersWithSession(NewAdmin, NewAdminSession, NewAdminSessionClient, controllers...)}
	return m
}

func (m *Manager) Add(ctx op_context.Context, login string, password string, phone string) (*Admin, error) {
	return m.UsersWithSessionBase.Add(ctx, login, password, user.Phone(phone, &Admin{}))
}
