package httpstrangler

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

func NewHandlerFromURLs(manager ProxyManager, defaultURL, alternativeURL *url.URL, options ...Option) http.Handler {
	defaultHandler := httputil.NewSingleHostReverseProxy(defaultURL)
	alternativeHandler := httputil.NewSingleHostReverseProxy(alternativeURL)

	return NewHandler(manager, defaultHandler, alternativeHandler, options...)
}

func NewHandler(manager ProxyManager, defaultHandler, alternativeHandler http.Handler, options ...Option) http.Handler {
	diffHandler := newDiffHandler(defaultHandler, alternativeHandler, options)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch manager.GetProxyMode(r) {
		case ProxyModeUseDefault:
			defaultHandler.ServeHTTP(w, r)
		case ProxyModeUseAlternative:
			alternativeHandler.ServeHTTP(w, r)
		case ProxyModeUseDefaultAndDiff:
			diffHandler.ServeHTTP(w, r)
		}
	})
}
