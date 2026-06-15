// Package initial is the package that starts the service to initialize the service, including
// the initialization configuration, service configuration, connecting to the database, and
// resource release needed when shutting down the service.
package initial

import (
	"flag"
	"strconv"
	"strings"
	"time"

	"github.com/go-dev-frame/sponge/pkg/logger"
	"github.com/go-dev-frame/sponge/pkg/stat"
	"github.com/go-dev-frame/sponge/pkg/tracer"

	"agent-base/services/user-service/configs"
	"agent-base/services/user-service/internal/config"
	"agent-base/services/user-service/internal/dao"
	"agent-base/services/user-service/internal/database"
	"agent-base/services/user-service/internal/pkg/ath"
	"agent-base/services/user-service/internal/pkg/code"
	"agent-base/services/user-service/internal/pkg/jwt"
)

var (
	version    string
	configFile string
)

// InitApp initial app configuration
func InitApp() {
	initConfig()
	cfg := config.Get()
	if cfg.App.Env == "prod" && !strings.HasPrefix(cfg.ATH.BaseURL, "https://") {
		panic("ath.baseURL must use https in production")
	}

	// initializing log
	_, err := logger.Init(
		logger.WithLevel(cfg.Logger.Level),
		logger.WithFormat(cfg.Logger.Format),
		logger.WithSave(
			cfg.Logger.IsSave,
			//logger.WithFileName(cfg.Logger.LogFileConfig.Filename),
			//logger.WithFileMaxSize(cfg.Logger.LogFileConfig.MaxSize),
			//logger.WithFileMaxBackups(cfg.Logger.LogFileConfig.MaxBackups),
			//logger.WithFileMaxAge(cfg.Logger.LogFileConfig.MaxAge),
			//logger.WithFileIsCompression(cfg.Logger.LogFileConfig.IsCompression),
		),
	)
	if err != nil {
		panic(err)
	}
	logger.Debug(config.Show("jwt.secret", "ath.anchor.authToken", "ath.signingKeys.authToken"))
	logger.Info("[logger] was initialized")

	// initializing tracing
	if cfg.App.EnableTrace {
		tracer.InitWithConfig(
			cfg.App.Name,
			cfg.App.Env,
			cfg.App.Version,
			cfg.Jaeger.AgentHost,
			strconv.Itoa(cfg.Jaeger.AgentPort),
			cfg.App.TracingSamplingRate,
		)
		logger.Info("[tracer] was initialized")
	}

	// initializing the print system and process resources
	if cfg.App.EnableStat {
		stat.Init(
			stat.WithLog(logger.Get()),
			stat.WithAlarm(), // invalid if it is windows, the default threshold for cpu and memory is 0.8, you can modify them
			stat.WithPrintField(logger.String("service_name", cfg.App.Name), logger.String("host", cfg.App.Host)),
		)
		logger.Info("[resource statistics] was initialized")
	}

	// initializing database
	database.InitDB()
	logger.Infof("[%s] was initialized", cfg.Database.Driver)
	database.InitCache(cfg.App.CacheType)
	if cfg.App.CacheType != "" {
		logger.Infof("[%s] was initialized", cfg.App.CacheType)
	}

	var signer *ath.ServerSigner
	if len(cfg.ATH.SigningKeys) > 0 {
		keys := make([]ath.SigningKeyConfig, 0, len(cfg.ATH.SigningKeys))
		for _, key := range cfg.ATH.SigningKeys {
			keys = append(keys, ath.SigningKeyConfig{
				ID: key.ID, KeyFile: key.KeyFile,
				PublicKeyFile:   key.PublicKeyFile,
				SigningEndpoint: key.SigningEndpoint,
				AuthToken:       key.AuthToken,
			})
		}
		signer, err = ath.LoadServerKeyRing(
			cfg.ATH.ServerDID, cfg.ATH.ActiveSigningKeyID, keys, cfg.App.Env != "prod",
		)
	} else {
		signer, err = ath.LoadServerSigner(
			cfg.ATH.ServerDID,
			cfg.ATH.SigningKeyID,
			cfg.ATH.SigningKeyFile,
			cfg.App.Env != "prod",
		)
	}
	if err != nil {
		panic(err)
	}
	handshakeTTL := time.Duration(cfg.ATH.HandshakeTTL) * time.Second
	handshakeService := ath.NewHandshakeService(
		ath.NewRedisHandshakeStore(database.GetRedisCli()),
		signer,
		handshakeTTL,
	)
	sessionTTL := time.Duration(cfg.OAuth.AccessTokenExpire) * time.Second
	handshakeService.SetSessionTTL(sessionTTL)
	ath.SetDefaultHandshakeService(handshakeService)
	ath.SetDefaultAuditService(ath.NewAuditService(
		dao.NewATHAuditRecordDao(database.GetDB()),
		signer,
	))
	var anchorClient ath.AnchorClient
	if cfg.ATH.Anchor.Endpoint != "" {
		anchorClient, err = ath.NewHTTPAnchorClient(
			cfg.ATH.Anchor.Endpoint,
			cfg.ATH.Anchor.AuthToken,
			time.Duration(cfg.ATH.Anchor.TimeoutSeconds)*time.Second,
			cfg.App.Env != "prod",
		)
		if err != nil {
			panic(err)
		}
	}
	ath.SetDefaultAnchorWorker(ath.NewAnchorWorker(
		dao.NewATHAuditOutboxDao(database.GetDB()),
		anchorClient,
		time.Duration(cfg.ATH.Anchor.IntervalSeconds)*time.Second,
		cfg.ATH.Anchor.BatchSize,
	))
	logger.Info("[ath handshake] was initialized")

	// initializing jwt
	jwt.SetConfig(cfg.JWT.Secret, cfg.JWT.Issuer, cfg.JWT.ExpireHours)
	logger.Info("[jwt] was initialized")

	// initializing code redis client
	if cfg.App.CacheType == "redis" {
		code.SetRedisClient(database.GetRedisCli())
		logger.Info("[code redis] was initialized")
	}
}

func initConfig() {
	flag.StringVar(&version, "version", "", "service Version Number")
	flag.StringVar(&configFile, "c", "", "configuration file")
	flag.Parse()

	getConfigFromLocal()

	if version != "" {
		config.Get().App.Version = version
	}
}

// get configuration from local configuration file
func getConfigFromLocal() {
	if configFile == "" {
		configFile = configs.Location("user_service.yml")
	}
	err := config.Init(configFile)
	if err != nil {
		panic("init config error: " + err.Error())
	}
}
