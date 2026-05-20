// Package routers is a package dedicated to registering routes, and supports both
// manual route registration and automatic route registration.
package routers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/go-dev-frame/sponge/pkg/errcode"
	"github.com/go-dev-frame/sponge/pkg/gin/handlerfunc"
	"github.com/go-dev-frame/sponge/pkg/gin/middleware"
	"github.com/go-dev-frame/sponge/pkg/gin/middleware/metrics"
	"github.com/go-dev-frame/sponge/pkg/gin/prof"
	"github.com/go-dev-frame/sponge/pkg/logger"

	"agent-base/services/user-service/docs"
	"agent-base/services/user-service/internal/config"
)

var (
	apiV1RouterFns []func(r *gin.RouterGroup) // group router functions
	// if you have other group routes you can define them here
	// example:
	//     apiV2RouterFns []func(r *gin.RouterGroup)
)

// NewRouter create a new router
func NewRouter() *gin.Engine {
	r := gin.New()

	r.Use(gin.Recovery())
	r.Use(middleware.Cors())

	if config.Get().HTTP.Timeout > 0 {
		// if you need more fine-grained control over your routes, set the timeout in your routes, unsetting the timeout globally here.
		r.Use(middleware.Timeout(time.Second * time.Duration(config.Get().HTTP.Timeout)))
	}

	// request id middleware
	r.Use(middleware.RequestID())

	// logger middleware, to print simple messages, replace middleware.Logging with middleware.SimpleLog
	r.Use(middleware.Logging(
		middleware.WithLog(logger.Get()),
		middleware.WithRequestIDFromContext(),
		middleware.WithIgnoreRoutes("/metrics"), // ignore path
	))

	// metrics middleware
	if config.Get().App.EnableMetrics {
		r.Use(metrics.Metrics(r,
			//metrics.WithMetricsPath("/metrics"),                // default is /metrics
			metrics.WithIgnoreStatusCodes(http.StatusNotFound), // ignore 404 status codes
		))
	}

	// limit middleware
	if config.Get().App.EnableLimit {
		r.Use(middleware.RateLimit(
		//middleware.WithWindow(time.Second*5), // default 10s
		//middleware.WithBucket(1000), // default 100
		//middleware.WithCPUThreshold(750), // default 800
		))
	}

	// circuit breaker middleware
	if config.Get().App.EnableCircuitBreaker {
		r.Use(middleware.CircuitBreaker(
			//middleware.WithBreakerOption(
			//circuitbreaker.WithSuccess(75),           // default 60
			//circuitbreaker.WithRequest(100),          // default 100
			//circuitbreaker.WithBucket(20),            // default 10
			//circuitbreaker.WithWindow(time.Second*3), // default 3s
			//),
			//middleware.WithDegradeHandler(handler),              // Add degradation processing
			middleware.WithValidCode( // Add error codes to trigger circuit breaking
				errcode.InternalServerError.Code(),
				errcode.ServiceUnavailable.Code(),
			),
		))
	}

	// trace middleware
	if config.Get().App.EnableTrace {
		r.Use(middleware.Tracing(config.Get().App.Name))
	}

	// profile performance analysis
	if config.Get().App.EnableHTTPProfile {
		prof.Register(r, prof.WithIOWaitTime())
	}

	r.GET("/health", handlerfunc.CheckHealth)
	r.GET("/ping", handlerfunc.Ping)
	r.GET("/codes", handlerfunc.ListCodes)

	if config.Get().App.Env != "prod" {
		r.GET("/config", gin.WrapF(errcode.ShowConfig([]byte(config.Show()))))
		// register swagger routes, generate code via swag init
		docs.SwaggerInfo.BasePath = ""
		// access path /swagger/index.html
		r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	// register routers, middleware support
	registerRouters(r, "/api/v1", apiV1RouterFns)
	// if you have other group routes you can add them here
	// example:
	//    registerRouters(r, "/api/v2", apiV2RouteFns, middleware.Auth())

	return r
}

func registerRouters(r *gin.Engine, groupPath string, routerFns []func(*gin.RouterGroup), handlers ...gin.HandlerFunc) {
	rg := r.Group(groupPath, handlers...)
	for _, fn := range routerFns {
		fn(rg)
	}
}
