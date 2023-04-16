package httpstrangler

import "net/http"

type ProxyManager interface {
	GetProxyMode(r *http.Request) ProxyMode
}

type ProxyMode int

const (
	ProxyModeUseDefault ProxyMode = iota
	ProxyModeUseAlternative
	ProxyModeUseDefaultAndDiff
)
