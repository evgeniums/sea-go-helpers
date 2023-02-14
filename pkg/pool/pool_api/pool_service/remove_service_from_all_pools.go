package pool_service

import (
	"github.com/evgeniums/go-backend-helpers/pkg/api"
	"github.com/evgeniums/go-backend-helpers/pkg/api/api_server"
)

type RemoveServiceFromAllPoolsEndpoint struct {
	PoolEndpoint
}

func (e *RemoveServiceFromAllPoolsEndpoint) HandleRequest(request api_server.Request) error {

	// setup
	c := request.TraceInMethod("pool.RemoveServiceFromAllPools")
	defer request.TraceOutMethod()

	// do operation
	serviceId := request.GetResourceId("service")
	err := e.service.Pools.RemoveServiceFromAllPools(request, serviceId)
	if err != nil {
		c.SetMessage("failed to remove services from all pools")
		return c.SetError(err)
	}

	// done
	return nil
}

func RemoveServiceFromAllPools(s *PoolService) *RemoveServiceFromAllPoolsEndpoint {
	e := &RemoveServiceFromAllPoolsEndpoint{}
	e.Construct(s, api.Unbind())
	return e
}
