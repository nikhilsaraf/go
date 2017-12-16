package main

import (
	"net/http"
	"os"
	"runtime"

	"github.com/go-chi/chi"
	"github.com/pkg/errors"
	"github.com/rs/cors"
	"github.com/spf13/cobra"
	"github.com/stellar/go/services/friendbot/internal"
	"github.com/stellar/go/support/config"
	"github.com/stellar/go/support/http/server"
	"github.com/stellar/go/support/log"
)

// Config represents the configuration of a friendbot server
type Config struct {
	Port              int               `toml:"port" valid:"required"`
	FriendbotSecret   string            `toml:"friendbot_secret" valid:"required"`
	NetworkPassphrase string            `toml:"network_passphrase" valid:"required"`
	HorizonURL        string            `toml:"horizon_url" valid:"required"`
	StartingBalance   string            `toml:"starting_balance" valid:"required"`
	TLS               *server.TLSConfig `valid:"optional"`
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	rootCmd := &cobra.Command{
		Use:   "friendbot",
		Short: "friendbot for the Stellar Test Network",
		Long:  "client-facing api server for the friendbot service on the Stellar Test Network",
		Run:   run,
	}

	rootCmd.PersistentFlags().String("conf", "./friendbot.cfg", "config file path")
	rootCmd.Execute()
}

func run(cmd *cobra.Command, args []string) {
	var (
		cfg     Config
		cfgPath = cmd.PersistentFlags().Lookup("conf").Value.String()
	)
	log.SetLevel(log.InfoLevel)
	err := config.Read(cfgPath, &cfg)
	if err != nil {
		switch cause := errors.Cause(err).(type) {
		case *config.InvalidConfigError:
			log.Error("config file: ", cause)
		default:
			log.Error(err)
		}
		os.Exit(1)
	}

	fb := initFriendbot(cfg.FriendbotSecret, cfg.NetworkPassphrase, cfg.HorizonURL, cfg.StartingBalance)
	router := initRouter(fb)

	server.Serve(router, cfg.Port, cfg.TLS)
}

func initRouter(fb *internal.Bot) *chi.Mux {
	routerConfig := server.EmptyConfig()

	// middleware
	server.AddBasicMiddleware(routerConfig)
	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedHeaders: []string{"*"},
	})
	routerConfig.Middleware(func(h http.Handler) http.Handler {
		return c.Handler(h)
	})

	// endpoints
	routerConfig.Route(http.MethodGet, "/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello World"))
	}))
	// routerConfig.Route(http.MethodPost, "/", &internal.FriendbotAction{Friendbot: fb})

	return server.NewRouter(routerConfig)
}
