package pool_console

import (
	"fmt"

	"github.com/evgeniums/go-backend-helpers/pkg/db"
	"github.com/evgeniums/go-backend-helpers/pkg/utils"
)

const DisableServiceCmd string = "disable_service"
const DisableServiceDescription string = "Disable service"

func DisableService() Handler {
	a := &DisableServiceHandler{}
	a.Init(DisableServiceCmd, DisableServiceDescription)
	return a
}

type DisableServiceData struct {
	Service string `long:"pool" description:"Short name of the service" required:"true"`
}

type DisableServiceHandler struct {
	HandlerBase
	DisableServiceData
}

func (a *DisableServiceHandler) Data() interface{} {
	return &a.DisableServiceData
}

func (a *DisableServiceHandler) Execute(args []string) error {

	ctx, controller, err := a.Context(a.Data())
	if err != nil {
		return err
	}
	defer ctx.Close()

	fields := db.Fields{}
	fields["active"] = false

	_, err = controller.UpdateService(ctx, a.Service, fields, true)
	if err == nil {
		pool, err := controller.FindPool(ctx, a.Service, true)
		if err == nil {
			if pool != nil {
				fmt.Printf("Updated service:\n\n%s\n\n", utils.DumpPrettyJson(pool))
			} else {
				fmt.Println("Pool not found")
			}
		}
	}
	return err
}
