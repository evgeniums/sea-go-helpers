package bare_bones_server

import (
	"github.com/evgeniums/go-backend-helpers/pkg/api/api_server"
	"github.com/evgeniums/go-backend-helpers/pkg/api/api_server/rest_api_gin_server"
	"github.com/evgeniums/go-backend-helpers/pkg/app_context"
	"github.com/evgeniums/go-backend-helpers/pkg/auth"
	"github.com/evgeniums/go-backend-helpers/pkg/auth/auth_methods/auth_factory"
	"github.com/evgeniums/go-backend-helpers/pkg/auth/auth_service"
	"github.com/evgeniums/go-backend-helpers/pkg/auth/auth_session"
	"github.com/evgeniums/go-backend-helpers/pkg/config/object_config"
	"github.com/evgeniums/go-backend-helpers/pkg/logger"
	"github.com/evgeniums/go-backend-helpers/pkg/multitenancy"
	"github.com/evgeniums/go-backend-helpers/pkg/pool/app_with_pools"
	"github.com/evgeniums/go-backend-helpers/pkg/signature"
	"github.com/evgeniums/go-backend-helpers/pkg/sms"
	"github.com/evgeniums/go-backend-helpers/pkg/utils"
)

type Server interface {
	ApiServer() api_server.Server
	Auth() auth.Auth
	SmsManager() sms.SmsManager
	SignatureManager() signature.SignatureManager
}

type Config struct {
	Auth             auth.Auth
	Server           api_server.Server
	SmsManager       sms.SmsManager
	SmsProviders     sms.ProviderFactory
	SignatureManager signature.SignatureManager

	WithoutStatusService bool
	WithoutDynamicTables bool

	MultitenancyBaseServices bool

	DefaultPoolServiceName    string
	DefaultPoolServiceType    string
	DefaultPrivatePoolService bool
}

type pimpl struct {
	auth             auth.Auth
	server           api_server.Server
	smsManager       sms.SmsManager
	smsProviders     sms.ProviderFactory
	users            auth_session.WithUserSessionManager
	signatureManager signature.SignatureManager
}

type BareBonesServerBaseConfig struct {
	POOL_SERVICE_NAME    string
	POOL_SERVICE_TYPE    string
	PRIVATE_POOL_SERVICE bool
}

type BareBonesServerBase struct {
	pimpl

	config BareBonesServerBaseConfig

	WithoutStatusService bool
	WithoutDynamicTables bool

	MultitenancyBaseServices bool
}

func (s *BareBonesServerBase) Config() interface{} {
	return &s.config
}

func (s *BareBonesServerBase) Construct(users auth_session.WithUserSessionManager, config ...Config) {
	s.pimpl.users = users
	if len(config) != 0 {
		cfg := config[0]
		s.pimpl.server = cfg.Server
		s.pimpl.auth = cfg.Auth
		s.pimpl.smsManager = cfg.SmsManager
		s.pimpl.smsProviders = cfg.SmsProviders
		s.pimpl.signatureManager = cfg.SignatureManager

		s.WithoutDynamicTables = cfg.WithoutDynamicTables
		s.WithoutStatusService = cfg.WithoutStatusService
		s.MultitenancyBaseServices = cfg.MultitenancyBaseServices

		s.config.POOL_SERVICE_NAME = cfg.DefaultPoolServiceName
		s.config.POOL_SERVICE_TYPE = cfg.DefaultPoolServiceType
		s.config.PRIVATE_POOL_SERVICE = cfg.DefaultPrivatePoolService
	}
}

func New(users auth_session.WithUserSessionManager, config ...Config) *BareBonesServerBase {
	s := &BareBonesServerBase{}
	s.Construct(users, config...)
	return s
}

func (s *BareBonesServerBase) Init(app app_context.Context, tenancyManager multitenancy.Multitenancy, configPath ...string) error {

	path := utils.OptionalArg("server", configPath...)

	// load configuration
	err := object_config.LoadLogValidate(app.Cfg(), app.Logger(), app.Validator(), s, path)
	if err != nil {
		return app.Logger().PushFatalStack("failed to load bare bone server configuration", err)
	}

	// init SMS manager
	if s.pimpl.smsManager == nil && s.pimpl.smsProviders != nil {
		smsManager := sms.NewSmsManager()
		err := smsManager.Init(app.Cfg(), app.Logger(), app.Validator(), s.pimpl.smsProviders, "sms")
		if err != nil {
			return app.Logger().PushFatalStack("failed to init SMS manager", err)
		}
		s.pimpl.smsManager = smsManager
	}

	// init auth controller
	if s.pimpl.auth == nil {
		auth := auth.NewAuth()
		authPath := object_config.Key(path, "auth")
		err = auth.Init(app.Cfg(), app.Logger(), app.Validator(), &auth_factory.DefaultAuthFactory{Users: s.pimpl.users, SmsManager: s.pimpl.smsManager, SignatureManager: s.pimpl.signatureManager}, authPath)
		if err != nil {
			return app.Logger().PushFatalStack("failed to init auth manager", err)
		}
		s.pimpl.auth = auth
	}

	// init REST API server
	if s.pimpl.server == nil {

		server := rest_api_gin_server.NewServer()
		err = s.initFromPoolService(app, server)
		if err != nil {
			return err
		}

		serverPath := object_config.Key(path, "rest_api_server")
		err = server.Init(app, s.pimpl.auth, tenancyManager, serverPath)
		if err != nil {
			return app.Logger().PushFatalStack("failed to init REST API server", err)
		}
		s.pimpl.server = server
	}

	// add services
	if !s.WithoutStatusService {
		api_server.AddServiceToServer(s.pimpl.server, api_server.NewStatusService(s.MultitenancyBaseServices))
	}
	if !s.WithoutDynamicTables {
		api_server.AddServiceToServer(s.pimpl.server, api_server.NewDynamicTablesService(s.MultitenancyBaseServices))
	}
	api_server.AddServiceToServer(s.pimpl.server, auth_service.NewAuthService(s.MultitenancyBaseServices))

	// done
	return nil
}

func (s *BareBonesServerBase) initFromPoolService(app app_context.Context, restApiServer *rest_api_gin_server.Server) error {

	if s.config.POOL_SERVICE_NAME != "" {

		poolApp, ok := app.(app_with_pools.AppWithPools)
		if !ok {
			return app.Logger().PushFatalStack("invalid application type, must be pool app", nil)
		}

		// check if app with self pool
		selfPool, err := poolApp.Pools().SelfPool()
		if err != nil {
			return app.Logger().PushFatalStack("self pool must be specified for API server", err)
		}

		// find service for role
		service, err := selfPool.ServiceByName(s.config.POOL_SERVICE_NAME)
		if err != nil {
			return app.Logger().PushFatalStack("failed to find service with specified name", err, logger.Fields{"name": s.config.POOL_SERVICE_NAME})
		}

		if service.TypeName() != s.config.POOL_SERVICE_TYPE {
			return app.Logger().PushFatalStack("invalid service type", err, logger.Fields{"name": s.config.POOL_SERVICE_NAME, "service_type": s.config.POOL_SERVICE_TYPE, "pool_service_type": service.TypeName()})
		}

		if service.Provider() != app.Application() {
			return app.Logger().PushFatalStack("invalid service type", err, logger.Fields{"name": s.config.POOL_SERVICE_NAME, "service_type": s.config.POOL_SERVICE_TYPE, "pool_service_type": service.TypeName()})
		}

		// load server configuration from service
		restApiServer.SetConfigFromPoolService(service, s.config.PRIVATE_POOL_SERVICE)
	}

	return nil
}

func (s *BareBonesServerBase) Auth() auth.Auth {
	return s.pimpl.auth
}

func (s *BareBonesServerBase) ApiServer() api_server.Server {
	return s.pimpl.server
}

func (s *BareBonesServerBase) SmsManager() sms.SmsManager {
	return s.pimpl.smsManager
}

func (s *BareBonesServerBase) SignatureManager() signature.SignatureManager {
	return s.pimpl.signatureManager
}
