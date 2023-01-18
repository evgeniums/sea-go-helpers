package auth_hmac

import (
	"errors"
	"net/http"

	"github.com/evgeniums/go-backend-helpers/pkg/auth"
	"github.com/evgeniums/go-backend-helpers/pkg/common"
	"github.com/evgeniums/go-backend-helpers/pkg/config"
	"github.com/evgeniums/go-backend-helpers/pkg/config/object_config"
	"github.com/evgeniums/go-backend-helpers/pkg/logger"
	"github.com/evgeniums/go-backend-helpers/pkg/utils"
	"github.com/evgeniums/go-backend-helpers/pkg/validator"
)

const HmacProtocol = "hmac"
const HmacParameter = "hmac"

type AuthHmacConfig struct {
	common.WithNameBaseConfig
}

type AuthHmac struct {
	auth.AuthHandlerBase
	AuthHmacConfig
}

func (a *AuthHmac) Config() interface{} {
	return a
}

func (a *AuthHmac) Init(cfg config.Config, log logger.Logger, vld validator.Validator, configPath ...string) error {

	err := object_config.LoadLogValidate(cfg, log, vld, a, "auth.methods.hmac", configPath...)
	if err != nil {
		return log.Fatal("failed to load configuration of HMAC auth handler", err)
	}
	return nil
}

func (a *AuthHmac) Protocol() string {
	return HmacProtocol
}

const ErrorCodeInvalidHmac = "hmac_invalid"

func (a *AuthHmac) ErrorDescriptions() map[string]string {
	m := map[string]string{
		ErrorCodeInvalidHmac: "Invalid HMAC",
	}
	return m
}

func (a *AuthHmac) ErrorProtocolCodes() map[string]int {
	m := map[string]int{
		ErrorCodeInvalidHmac: http.StatusUnauthorized,
	}
	return m
}

// Check HMAC in request.
// Call this handler after discovering user (ctx.AuthUser() must be not nil).
// HMAC secret must be set for the user.
// HMAC string is calculated as BASE64(HMAC_SHA256(RequestMethod,RequestPath,RequestContent)), where BASE64 is calculated with padding.
func (a *AuthHmac) Handle(ctx auth.AuthContext) (bool, error) {

	// setup
	c := ctx.TraceInMethod("AuthHmac.Handle")
	var err error
	onExit := func() {
		if err != nil {
			c.SetError(err)
		}
		ctx.TraceOutMethod()
	}
	defer onExit()

	// get token from request
	requestHmac := ctx.GetAuthParameter(a.Protocol(), HmacParameter)
	if requestHmac == "" {
		return false, nil
	}

	// get secret from user
	if ctx.AuthUser() == nil {
		err := errors.New("unknown user")
		ctx.SetGenericErrorCode(auth.ErrorCodeUnauthorized)
		return true, err
	}
	secret := ctx.AuthUser().GetAuthParameter(a.Protocol(), "secret")
	if secret == "" {
		err := errors.New("hmac secret is not set for user")
		ctx.SetGenericErrorCode(auth.ErrorCodeUnauthorized)
		return true, err
	}

	// check hmac
	hmac := utils.NewHmac(secret)
	hmac.Calc([]byte(ctx.GetRequestMethod()), []byte(ctx.GetRequestPath()), ctx.GetRequestContent())
	err = hmac.CheckStr(requestHmac)
	if err != nil {
		ctx.SetGenericErrorCode(ErrorCodeInvalidHmac)
		return true, err
	}

	// done
	return true, nil
}
