package server

import (
	"net/http"

	"github.com/go-chi/chi"
)

// routeMaker translates the HTTP string method to the chi-equivalent route operation
var routeMaker = map[string]func(*chi.Mux, string, http.HandlerFunc){
	http.MethodGet: func(mux *chi.Mux, route string, fn http.HandlerFunc) {
		mux.Get(route, fn)
	},
	http.MethodPost: func(mux *chi.Mux, route string, fn http.HandlerFunc) {
		mux.Post(route, fn)
	},
}

// NewRouter creates a new router with the provided config
func NewRouter(c *Config) *chi.Mux {
	mux := chi.NewRouter()

	// add middleware
	mux.Use(c.middlewares...)

	// add routes
	for method, routes := range c.router {
		bindFn := routeMaker[method]
		for _, route := range routes {
			bindFn(mux, route.path, route.handler)
		}
	}

	return mux
}
