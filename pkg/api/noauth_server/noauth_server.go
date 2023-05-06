package noauth_server

import (
	"github.com/evgeniums/go-backend-helpers/pkg/api/api_server"
	"github.com/evgeniums/go-backend-helpers/pkg/api/api_server/rest_api_gin_server"
	"github.com/evgeniums/go-backend-helpers/pkg/auth"
	"github.com/evgeniums/go-backend-helpers/pkg/config/object_config"
	"github.com/evgeniums/go-backend-helpers/pkg/logger"
	"github.com/evgeniums/go-backend-helpers/pkg/multitenancy/app_with_multitenancy"
	"github.com/evgeniums/go-backend-helpers/pkg/pool"
	"github.com/evgeniums/go-backend-helpers/pkg/utils"
)

type Server interface {
	ApiServer() api_server.Server
	Auth() auth.Auth
}

type NoAuthServerConfig struct {
	POOL_SERVICE_NAME   string
	POOL_SERVICE_TYPE   string
	PUBLIC_POOL_SERVICE bool
}

type NoAuthServer struct {
	auth   auth.Auth
	server api_server.Server

	config        NoAuthServerConfig
	restApiServer *rest_api_gin_server.Server
}

type Config struct {
	Auth                     auth.Auth
	Server                   api_server.Server
	DefaultPoolServiceName   string
	DefaultPoolServiceType   string
	DefaultPublicPoolService bool
}

func New(config ...Config) *NoAuthServer {
	s := &NoAuthServer{}
	s.Construct(config...)
	return s
}

func (s *NoAuthServer) Config() interface{} {
	return &s.config
}

func (s *NoAuthServer) Construct(config ...Config) {
	if len(config) != 0 {
		cfg := config[0]
		s.server = cfg.Server
		s.config.POOL_SERVICE_TYPE = cfg.DefaultPoolServiceType
		s.config.POOL_SERVICE_NAME = cfg.DefaultPoolServiceName
		s.config.PUBLIC_POOL_SERVICE = cfg.DefaultPublicPoolService
		s.auth = cfg.Auth
	}

	// noauth
	if s.auth == nil {
		s.auth = auth.NewNoAuth()
	}

	// create REST API server
	if s.server == nil {
		s.restApiServer = rest_api_gin_server.NewServer()
		s.server = s.restApiServer
	}
}

func (s *NoAuthServer) Init(app app_with_multitenancy.AppWithMultitenancy, configPath ...string) error {

	path := utils.OptionalArg("server", configPath...)

	err := object_config.LoadLogValidate(app.Cfg(), app.Logger(), app.Validator(), s, path)
	if err != nil {
		return app.Logger().PushFatalStack("failed to load server configuration", err)
	}

	// init REST API server
	if s.restApiServer != nil {

		if s.config.POOL_SERVICE_NAME != "" {

			app.Logger().Info("Using configuration of pool service", logger.Fields{"service_name": s.config.POOL_SERVICE_NAME})

			// check if app with self pool
			selfPool, err := app.Pools().SelfPool()
			if err != nil {
				return app.Logger().PushFatalStack("self pool must be specified for api server", err)
			}

			// find service by name
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
			s.restApiServer.SetConfigFromPoolService(service, s.config.PUBLIC_POOL_SERVICE)
		}

		serverPath := object_config.Key(path, "rest_api_server")
		err := s.restApiServer.Init(app, s.auth, app.Multitenancy(), serverPath)
		if err != nil {
			return app.Logger().PushFatalStack("failed to init REST API server", err)
		}
	}

	// done
	return nil
}

func (s *NoAuthServer) SetConfigFromPoolService(service pool.PoolService, public ...bool) {
	if s.restApiServer != nil {
		s.restApiServer.SetConfigFromPoolService(service, public...)
	}
}

func (s *NoAuthServer) Auth() auth.Auth {
	return s.auth
}

func (s *NoAuthServer) ApiServer() api_server.Server {
	return s.server
}
