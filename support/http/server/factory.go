package server

import (
	"net/http"

	"github.com/go-chi/chi/middleware"
)

// EmptyConfig gives you a new empty Config
func EmptyConfig() *Config {
	router := make(map[string][]route)
	for _, method := range supportedMethods {
		router[method] = []route{}
	}

	return &Config{
		router:      router,
		middlewares: []func(http.Handler) http.Handler{},
	}
}

// AddBasicMiddleware is a helper function that augments the passed in Config with some basic middleware components
func AddBasicMiddleware(c *Config) {
	c.Middleware(middleware.RequestID)
	c.Middleware(middleware.Recoverer)
	c.Middleware(BindLoggerMiddleware(middleware.RequestIDKey))
}
