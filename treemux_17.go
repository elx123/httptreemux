//go:build go1.7
// +build go1.7

package httptreemux

import (
	"context"
	"net/http"
	"sync"
)

type TreeMux struct {
	root  *node
	mutex sync.RWMutex

	Group

	// The default PanicHandler just returns a 500 code.
	PanicHandler PanicHandler

	// The default NotFoundHandler is http.NotFound.
	NotFoundHandler func(w http.ResponseWriter, r *http.Request)

	// Any OPTIONS request that matches a path without its own OPTIONS handler will use this handler,
	// if set, instead of calling MethodNotAllowedHandler.
	OptionsHandler HandlerFunc

	// MethodNotAllowedHandler is called when a pattern matches, but that
	// pattern does not have a handler for the requested method. The default
	// handler just writes the status code http.StatusMethodNotAllowed and adds
	// the required Allowed header.
	// The methods parameter contains the map of each method to the corresponding
	// handler function.
	MethodNotAllowedHandler func(w http.ResponseWriter, r *http.Request,
		methods map[string]HandlerFunc)

	// HeadCanUseGet allows the router to use the GET handler to respond to
	// HEAD requests if no explicit HEAD handler has been added for the
	// matching pattern. This is true by default.
	HeadCanUseGet bool

	// RedirectCleanPath allows the router to try clean the current request path,
	// if no handler is registered for it, using CleanPath from github.com/dimfeld/httppath.
	// This is true by default.
	RedirectCleanPath bool

	// RedirectTrailingSlash enables automatic redirection in case router doesn't find a matching route
	// for the current request path but a handler for the path with or without the trailing
	// slash exists. This is true by default.
	RedirectTrailingSlash bool

	// RemoveCatchAllTrailingSlash removes the trailing slash when a catch-all pattern
	// is matched, if set to true. By default, catch-all paths are never redirected.
	RemoveCatchAllTrailingSlash bool

	// RedirectBehavior sets the default redirect behavior when RedirectTrailingSlash or
	// RedirectCleanPath are true. The default value is Redirect301.
	RedirectBehavior RedirectBehavior

	// RedirectMethodBehavior overrides the default behavior for a particular HTTP method.
	// The key is the method name, and the value is the behavior to use for that method.
	RedirectMethodBehavior map[string]RedirectBehavior

	// PathSource determines from where the router gets its path to search.
	// By default it pulls the data from the RequestURI member, but this can
	// be overridden to use URL.Path instead.
	//
	// There is a small tradeoff here. Using RequestURI allows the router to handle
	// encoded slashes (i.e. %2f) in the URL properly, while URL.Path provides
	// better compatibility with some utility functions in the http
	// library that modify the Request before passing it to the router.
	PathSource PathSource

	// EscapeAddedRoutes controls URI escaping behavior when adding a route to the tree.
	// If set to true, the router will add both the route as originally passed, and
	// a version passed through URL.EscapedPath. This behavior is disabled by default.
	EscapeAddedRoutes bool

	// If present, override the default context with this one.
	DefaultContext context.Context

	// SafeAddRoutesWhileRunning tells the router to protect all accesses to the tree with an RWMutex. This is only needed
	// if you are going to add routes after the router has already begun serving requests. There is a potential
	// performance penalty at high load.
	SafeAddRoutesWhileRunning bool

	// CaseInsensitive determines if routes should be treated as case-insensitive.
	CaseInsensitive bool
}

func (t *TreeMux) setDefaultRequestContext(r *http.Request) *http.Request {
	if t.DefaultContext != nil {
		r = r.WithContext(t.DefaultContext)
	}

	return r
}

// 这里有点绕,初看不怎么理解,简单来说就是通过构造treemux,然后利用treemux 再构造contextgroup
type ContextMux struct {
	*TreeMux
	*ContextGroup
}

// NewContextMux returns a TreeMux preconfigured to work with standard http
// Handler functions and context objects.
func NewContextMux() *ContextMux {
	mux := New()
	//embed struct 也会分配内存
	cg := mux.UsingContext()

	return &ContextMux{
		TreeMux:      mux,
		ContextGroup: cg,
	}
}

func (cm *ContextMux) NewGroup(path string) *ContextGroup {
	return cm.ContextGroup.NewGroup(path)
}

// GET is convenience method for handling GET requests on a context group.
func (cm *ContextMux) GET(path string, handler http.HandlerFunc) {
	cm.ContextGroup.Handle("GET", path, handler)
}

// POST is convenience method for handling POST requests on a context group.
func (cm *ContextMux) POST(path string, handler http.HandlerFunc) {
	cm.ContextGroup.Handle("POST", path, handler)
}

// PUT is convenience method for handling PUT requests on a context group.
func (cm *ContextMux) PUT(path string, handler http.HandlerFunc) {
	cm.ContextGroup.Handle("PUT", path, handler)
}

// DELETE is convenience method for handling DELETE requests on a context group.
func (cm *ContextMux) DELETE(path string, handler http.HandlerFunc) {
	cm.ContextGroup.Handle("DELETE", path, handler)
}

// PATCH is convenience method for handling PATCH requests on a context group.
func (cm *ContextMux) PATCH(path string, handler http.HandlerFunc) {
	cm.ContextGroup.Handle("PATCH", path, handler)
}

// HEAD is convenience method for handling HEAD requests on a context group.
func (cm *ContextMux) HEAD(path string, handler http.HandlerFunc) {
	cm.ContextGroup.Handle("HEAD", path, handler)
}

// OPTIONS is convenience method for handling OPTIONS requests on a context group.
func (cm *ContextMux) OPTIONS(path string, handler http.HandlerFunc) {
	cm.ContextGroup.Handle("OPTIONS", path, handler)
}
