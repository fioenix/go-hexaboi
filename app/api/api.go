package api

import (
	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/favicon"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/spf13/cobra"

	"hecigo.com/go-hexaboi/app/api/handler"
	"hecigo.com/go-hexaboi/app/api/middleware"
	"hecigo.com/go-hexaboi/app/api/router"
	"hecigo.com/go-hexaboi/infra/core"
	"hecigo.com/go-hexaboi/infra/elasticsearch"
	"hecigo.com/go-hexaboi/infra/orientdb"
	"hecigo.com/go-hexaboi/infra/postgres"
	"hecigo.com/go-hexaboi/infra/redis"

	_ "hecigo.com/go-hexaboi/docs"
)

type API struct {
	App          *fiber.App
	Profile      string
	IsProduction bool
}

var listen string

func Register(rootApp string, env string, rootCmd *cobra.Command) {
	var selfCmd = &cobra.Command{
		Use:     "serve",
		Short:   "Start " + rootApp + " RESTful API",
		Long:    rootApp + ` RESTful API provides inventory data and requests for other services`,
		Example: "gohexaboi serve -l localhost:3000",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Initialize new API
			api := New(env)

			// Open PostgreSQL connections
			postgres.OpenDefaultConnection()
			defer postgres.CloseAll()

			// Initialize Elasticsearch clients
			elasticsearch.OpenDefaultConnection()

			// Initialize Redis clients
			redis.OpenDefaultConnection()
			redis.EnableSession() // Enable session store for Redis
			defer redis.CloseAll()

			// Open OrientDB connections
			orientdb.OpenDefaultConnection()
			defer orientdb.CloseAll()

			// Listen serves HTTP requests from the given addr
			return api.App.Listen(listen)
		},
	}
	selfCmd.Flags().StringVarP(&listen, "listen", "l", "localhost:3000", "Listen serves HTTP requests from the given addr")

	rootCmd.AddCommand(selfCmd)
}

func New(env string) *API {

	// go run app.go -prod
	isProduction := (env == "prod" || env == "production")

	// Create a new app
	app := fiber.New(fiber.Config{
		AppName:       core.AppName() + " " + core.AppVersion(),
		StrictRouting: false,
		CaseSensitive: false,
		Prefork:       isProduction,
		JSONEncoder:   json.Marshal,
		JSONDecoder:   json.Unmarshal,
		ErrorHandler:  handler.DefaultError,
	})

	// Initialize middlewares
	app.Use(favicon.New(favicon.Config{
		File: "./favicon.ico",
	}))

	app.Use(recover.New(recover.Config{
		EnableStackTrace: true,
	}))

	if core.GetBoolEnv("COMPRESS_ENABLE", false) {
		(&middleware.Compress{}).Enable(app)
	}

	if core.GetBoolEnv("CORS_ENABLE", true) {
		(&middleware.CORS{}).Enable(app)
	}

	// App health check. It must be by pass some middlewares.
	app.Get("/", handler.HealthCheck)

	if core.GetBoolEnv("CACHE_ENABLE", false) {
		(&middleware.Cache{}).Enable(app)
	}

	if core.GetBoolEnv("PPROF_ENABLE", false) {
		(&middleware.Pprof{}).Enable(app)
	}

	if core.GetBoolEnv("HTTP_LOG_ENABLE", false) {
		(&middleware.HttpLogger{}).Enable(app)
	}

	// Create a /v1 endpoint. Just replaces if the frontend is already.
	root := app.Group(core.AppRootPath())
	router.RegisterDefaultRouter(root)

	// Always response NotFound at the end of routes
	app.Use(handler.NotFound)

	return &API{App: app, Profile: env, IsProduction: isProduction}
}
