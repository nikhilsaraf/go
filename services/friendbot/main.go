package main

import (
	"fmt"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/PuerkitoBio/throttled"
	"github.com/pkg/errors"
	metrics "github.com/rcrowley/go-metrics"
	"github.com/rs/cors"
	"github.com/spf13/cobra"
	"github.com/stellar/go/services/friendbot/internal"
	hm "github.com/stellar/go/services/horizon/middleware"
	"github.com/stellar/go/support/config"
	"github.com/stellar/go/support/log"
	"github.com/tylerb/graceful"
	"github.com/zenazn/goji/web"
	"github.com/zenazn/goji/web/middleware"
	"golang.org/x/net/context"
	"golang.org/x/net/http2"
)

// TLSConfig specifies the TLS portion of the config
type TLSConfig struct {
	CertificateFile string `toml:"certificate-file" valid:"required"`
	PrivateKeyFile  string `toml:"private-key-file" valid:"required"`
}

// Config represents the configuration of a friendbot server
type Config struct {
	Port              int        `toml:"port" valid:"required"`
	FriendbotSecret   string     `toml:"friendbot_secret" valid:"required"`
	NetworkPassphrase string     `toml:"network_passphrase" valid:"required"`
	HorizonURL        string     `toml:"horizon_url" valid:"required"`
	StartingBalance   string     `toml:"starting_balance" valid:"required"`
	TLS               *TLSConfig `valid:"optional"`
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

	appWeb := initWeb()
	fb := initFriendbot(cfg.FriendbotSecret, cfg.NetworkPassphrase, cfg.HorizonURL, cfg.StartingBalance)
	initRouter(appWeb.router, fb)

	Serve(appWeb.router, cfg.Port, cfg.TLS)
}

// Web contains the http server related fields for horizon: the router,
// rate limiter, etc.
type Web struct {
	router      *web.Mux
	rateLimiter *throttled.Throttler

	requestTimer metrics.Timer
	failureMeter metrics.Meter
	successMeter metrics.Meter
}

// initWeb installed a new Web instance onto the provided app object.
func initWeb() *Web {
	return &Web{
		router:       web.New(),
		requestTimer: metrics.NewTimer(),
		failureMeter: metrics.NewMeter(),
		successMeter: metrics.NewMeter(),
	}
}

func initRouter(r *web.Mux, fb *internal.Bot) {
	ctx, _ := context.WithCancel(context.Background())

	// middleware
	r.Use(hm.StripTrailingSlashMiddleware())
	r.Use(hm.ContextMiddleware(ctx))
	r.Use(middleware.RequestID)
	r.Use(hm.LoggerMiddleware)
	r.Use(hm.RecoverMiddleware)
	r.Use(middleware.AutomaticOptions)

	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedHeaders: []string{"*"},
	})
	r.Use(c.Handler)

	// friendbot
	r.Post("/", &internal.FriendbotAction{Friendbot: fb})
	r.Get("/", &internal.FriendbotAction{Friendbot: fb})
}

// Serve starts the friendbot web server, binding it to a socket, setting up the shutdown signals.
func Serve(router *web.Mux, port int, tls *TLSConfig) {
	router.Compile()
	http.Handle("/", router)

	addr := fmt.Sprintf(":%d", port)

	srv := &graceful.Server{
		Timeout: 10 * time.Second,

		Server: &http.Server{
			Addr:    addr,
			Handler: http.DefaultServeMux,
		},

		ShutdownInitiated: func() {
			log.Info("received signal, gracefully stopping")
		},
	}

	http2.ConfigureServer(srv.Server, nil)

	log.Info("Starting friendbot on " + addr)

	var err error
	if tls != nil {
		err = srv.ListenAndServeTLS(tls.CertificateFile, tls.PrivateKeyFile)
	} else {
		err = srv.ListenAndServe()
	}

	if err != nil {
		log.Panic(err)
	}

	log.Info("stopped")
}
