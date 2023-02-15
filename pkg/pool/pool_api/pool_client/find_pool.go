package pool_client

import (
	"github.com/evgeniums/go-backend-helpers/pkg/api"
	"github.com/evgeniums/go-backend-helpers/pkg/api/api_client"
	"github.com/evgeniums/go-backend-helpers/pkg/op_context"
	"github.com/evgeniums/go-backend-helpers/pkg/pool"
	"github.com/evgeniums/go-backend-helpers/pkg/pool/pool_api"
)

type FindPool struct {
	result *pool_api.PoolResponse
}

func (a *FindPool) Exec(client api_client.Client, ctx op_context.Context, operation api.Operation) error {

	c := ctx.TraceInMethod("FindPool.Exec")
	defer ctx.TraceOutMethod()

	err := client.Exec(ctx, operation, nil, a.result)
	c.SetError(err)
	return err
}

func (p *PoolClient) FindPool(ctx op_context.Context, id string, idIsName ...bool) (pool.Pool, error) {

	// setup
	var err error
	c := ctx.TraceInMethod("PoolClient.FindPool")
	onExit := func() {
		if err != nil {
			c.SetError(err)
		}
		ctx.TraceOutMethod()
	}
	defer onExit()

	pId, pool, err := p.poolId(ctx, id, idIsName...)
	if err != nil {
		return nil, err
	}
	if pool != nil {
		return pool, nil
	}

	// prepare and exec handler
	handler := &FindPool{}
	err = api.NamedResourceOperation(p.ServiceResource, poolIdType, pId, pool_api.FindPool()).Exec(ctx, api_client.MakeOperationHandler(p.Client(), handler))
	if err != nil {
		c.SetMessage("failed to exec operation")
		return nil, err
	}

	// done
	return handler.result.PoolBase, nil
}
