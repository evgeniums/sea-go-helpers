package auth_test

import (
	"net/http"
	"testing"

	"github.com/evgeniums/go-backend-helpers/pkg/auth_methods/auth_sms"
	"github.com/evgeniums/go-backend-helpers/pkg/test_utils"
	"github.com/evgeniums/go-backend-helpers/pkg/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type Cmd struct {
	Param1 string
	Param2 string
}

func TestSms(t *testing.T) {
	app, users, server, opCtx := initOpTest(t)
	defer app.Close()

	// create user1
	login1 := "user1@example.com"
	password1 := "password1"
	user1, err := users.Add(opCtx, login1, password1, user.Phone("12345678", &User{}), user.Email("user1@example.com", &User{}))
	require.NoErrorf(t, err, "failed to add user")
	require.NotNil(t, user1)

	// prepare client
	client := test_utils.PrepareHttpClient(t, server.RestApiServer.GinEngine())

	// good login
	client.Login(t, login1, password1)

	// send with SMS confirmation
	assert.Empty(t, auth_sms.LastSmsCode)
	resp := client.Post(t, "/status/sms", nil)
	test_utils.CheckResponse(t, resp, &test_utils.Expected{HttpCode: http.StatusOK})
	assert.NotEmpty(t, auth_sms.LastSmsCode)

	// send altered data or path
	cmd1 := &Cmd{Param1: "value1_1", Param2: "value1_2"}
	cmd2 := &Cmd{Param1: "value2_1", Param2: "value2_2"}
	client.AutoSms = false
	resp = client.Post(t, "/status/sms", cmd1)
	test_utils.CheckResponse(t, resp, &test_utils.Expected{HttpCode: http.StatusUnauthorized, Error: auth_sms.ErrorCodeSmsConfirmationRequired})
	smsToken := resp.Object.Header().Get("x-auth-sms-token")
	smsDelay := resp.Object.Header().Get("x-auth-sms-delay")
	t.Logf("Current delay: %s", smsDelay)
	resp = client.SendSmsConfirmation(t, resp, auth_sms.LastSmsCode, http.MethodPost, "/status/sms", cmd2)
	test_utils.CheckResponse(t, resp, &test_utils.Expected{HttpCode: http.StatusUnauthorized, Error: auth_sms.ErrorCodeContentMismatch})
	headers := map[string]string{"x-auth-sms-token": smsToken}
	resp = client.SendSmsConfirmation(t, resp, auth_sms.LastSmsCode, http.MethodPost, "/status/sms-alt", cmd1, headers)
	test_utils.CheckResponse(t, resp, &test_utils.Expected{HttpCode: http.StatusUnauthorized, Error: auth_sms.ErrorCodeContentMismatch})
	resp = client.SendSmsConfirmation(t, resp, auth_sms.LastSmsCode, http.MethodPost, "/status/sms", cmd1, headers)
	test_utils.CheckResponse(t, resp, &test_utils.Expected{HttpCode: http.StatusOK})

	// TODO check altered method

	// check too many tries of invalid code
	client.AutoSms = false
	resp = client.Post(t, "/status/sms", nil)
	test_utils.CheckResponse(t, resp, &test_utils.Expected{HttpCode: http.StatusUnauthorized, Error: auth_sms.ErrorCodeSmsConfirmationRequired})
	resp = client.SendSmsConfirmation(t, resp, "0000", http.MethodPost, "/status/sms", nil)
	test_utils.CheckResponse(t, resp, &test_utils.Expected{HttpCode: http.StatusUnauthorized, Error: auth_sms.ErrorCodeInvalidSmsCode})
	resp = client.SendSmsConfirmation(t, resp, "1111", http.MethodPost, "/status/sms", nil)
	test_utils.CheckResponse(t, resp, &test_utils.Expected{HttpCode: http.StatusUnauthorized, Error: auth_sms.ErrorCodeInvalidSmsCode})
	resp = client.SendSmsConfirmation(t, resp, "2222", http.MethodPost, "/status/sms", nil)
	test_utils.CheckResponse(t, resp, &test_utils.Expected{HttpCode: http.StatusUnauthorized, Error: auth_sms.ErrorCodeTooManyTries})

	// check delay
	client.AutoSms = false
	resp = client.Post(t, "/status/sms", nil)
	test_utils.CheckResponse(t, resp, &test_utils.Expected{HttpCode: http.StatusUnauthorized, Error: auth_sms.ErrorCodeWaitDelay})
	smsDelay = resp.Object.Header().Get("x-auth-sms-delay")
	t.Logf("Current delay: %s", smsDelay)
	assert.Equal(t, "2", smsDelay)
	client.Sleep(t, 1, "SMS delay")
	client.AutoSms = false
	resp = client.Post(t, "/status/sms", nil)
	test_utils.CheckResponse(t, resp, &test_utils.Expected{HttpCode: http.StatusUnauthorized, Error: auth_sms.ErrorCodeWaitDelay})
	smsDelay = resp.Object.Header().Get("x-auth-sms-delay")
	t.Logf("Current delay: %s", smsDelay)
	assert.Equal(t, "1", smsDelay)
	client.Sleep(t, 2, "SMS delay")
	client.AutoSms = false
	resp = client.Post(t, "/status/sms", nil)
	test_utils.CheckResponse(t, resp, &test_utils.Expected{HttpCode: http.StatusUnauthorized, Error: auth_sms.ErrorCodeSmsConfirmationRequired})
	resp = client.SendSmsConfirmation(t, resp, auth_sms.LastSmsCode, http.MethodPost, "/status/sms", nil)
	test_utils.CheckResponse(t, resp, &test_utils.Expected{HttpCode: http.StatusOK})

	// check token expiration
	client.AutoSms = false
	resp = client.Post(t, "/status/sms", nil)
	test_utils.CheckResponse(t, resp, &test_utils.Expected{HttpCode: http.StatusUnauthorized, Error: auth_sms.ErrorCodeSmsConfirmationRequired})
	client.Sleep(t, 4, "SMS token expiration")
	resp = client.SendSmsConfirmation(t, resp, auth_sms.LastSmsCode, http.MethodPost, "/status/sms", nil)
	test_utils.CheckResponse(t, resp, &test_utils.Expected{HttpCode: http.StatusUnauthorized, Error: auth_sms.ErrorCodeTokenExpired})

	// check no SMS token
	client.AutoSms = false
	resp = client.Post(t, "/status/sms", nil)
	test_utils.CheckResponse(t, resp, &test_utils.Expected{HttpCode: http.StatusUnauthorized, Error: auth_sms.ErrorCodeSmsConfirmationRequired})
	headers = map[string]string{"x-auth-sms-code": auth_sms.LastSmsCode}
	resp = client.Post(t, "/status/sms", nil, headers)
	test_utils.CheckResponse(t, resp, &test_utils.Expected{HttpCode: http.StatusUnauthorized, Error: auth_sms.ErrorCodeSmsTokenRequired})

	// check invalid SMS token
	client.Sleep(t, 3, "SMS delay")
	client.AutoSms = false
	resp = client.Post(t, "/status/sms", nil)
	test_utils.CheckResponse(t, resp, &test_utils.Expected{HttpCode: http.StatusUnauthorized, Error: auth_sms.ErrorCodeSmsConfirmationRequired})
	headers = map[string]string{"x-auth-sms-code": auth_sms.LastSmsCode, "x-auth-sms-token": "blabla"}
	resp = client.Post(t, "/status/sms", nil, headers)
	test_utils.CheckResponse(t, resp, &test_utils.Expected{HttpCode: http.StatusUnauthorized, Error: auth_sms.ErrorCodeInvalidToken})
}
