package confirmation_control_api

import (
	"fmt"

	"github.com/evgeniums/go-backend-helpers/pkg/api"
	"github.com/evgeniums/go-backend-helpers/pkg/auth"
	"github.com/evgeniums/go-backend-helpers/pkg/generic_error"
)

const ServiceName string = "confirmation"

const OperationResource string = "operation"

func CheckConfirmation() api.Operation {
	return api.Post("check_confirmation")
}

func PrepareCheckConfirmation() api.Operation {
	return api.Get("prepare_check_confirmation")
}

func PrepareOperation() api.Operation {
	return api.Post("prepare_operation")
}

type Operation struct {
	Id        string `json:"id" validate:"required,id" vmessage:"Operation ID must be specified"`
	Recipient string `json:"phone" validate:"required" vmessage:"Recipient must be specified"`
	FailedUrl string `json:"failed_url" validate:"required,url" vmessage:"Invalid format of failed URL"`
}

type CodeCmd struct {
	Code string `json:"code" validate:"required" vmessage:"Code must be specified"`
}

type PrepareOperationResponse struct {
	api.ResponseStub
	Url string `json:"url"`
}

type PrepareCheckConfirmationResponse struct {
	api.ResponseStub
	FailedUrl string `json:"failed_url"`
}

type OperationCacheToken struct {
	Id        string `json:"id"`
	Recipient string `json:"recipient"`
	FailedUrl string `json:"failed_url"`
}

const ConfirmationCacheKey = "confirmation_service"

func OperationIdCacheKey(operationId string) string {
	return fmt.Sprintf("%s/%s", ConfirmationCacheKey, operationId)
}

func GetTokenFromCache(ctx auth.AuthContext) (*OperationCacheToken, error) {

	// setup
	c := ctx.TraceInMethod("GetTokenFromCache")
	defer ctx.TraceOutMethod()

	// get token from cache
	operationId := ctx.GetResourceId(OperationResource)
	ctx.SetLoggerField("cache_operation_id", operationId)
	cacheToken := &OperationCacheToken{}
	cacheKey := OperationIdCacheKey(operationId)
	found, err := ctx.Cache().Get(cacheKey, cacheToken)
	if err != nil {
		c.SetMessage("failed to get cache token")
		ctx.SetGenericErrorCode(generic_error.ErrorCodeInternalServerError)
		return nil, err
	}
	if !found {
		c.SetMessage("cache token not found")
		ctx.SetGenericErrorCode(generic_error.ErrorCodeNotFound)
		return nil, err
	}

	// done
	return cacheToken, nil
}
