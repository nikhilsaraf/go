package server

import (
	"net/http"
)

var supportedMethods = []string{
	http.MethodGet,
	http.MethodPost,
}

type route struct {
	path    string
	handler http.HandlerFunc
}

// Config is the immutable Config file that will be used to construct a server
type Config struct {
	router      map[string][]route // use a []route to maintain a consistent traversal ordering guarantee
	middlewares []func(http.Handler) http.Handler
}

// Route allows you to set (or override) an existing route by the HTTP method
func (c *Config) Route(method string, path string, handler http.HandlerFunc) {
	c.router[method] = append(c.router[method], route{
		path:    path,
		handler: handler,
	})
}

// Middleware adds a middleware to the list
func (c *Config) Middleware(m func(http.Handler) http.Handler) {
	c.middlewares = append(c.middlewares, m)
}
