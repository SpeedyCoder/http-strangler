package httpstrangler

import (
	"fmt"
	"github.com/google/go-cmp/cmp"
	"net/http"
)

type Response struct {
	Headers    http.Header
	StatusCode int
	Body       any
}

type Differ interface {
	Diff(defaultResp, alternativeResp *Response) string
}

type DiffReporter interface {
	ReportMatch(req *http.Request)
	ReportDiff(req *http.Request, diff string)
	ReportError(req *http.Request, err error)
}

type CMPDiffer struct{}

func (C CMPDiffer) Diff(defaultResp, alternativeResp *Response) string {
	return cmp.Diff(defaultResp, alternativeResp)
}

type PrintDiffReporter struct{}

func (p PrintDiffReporter) ReportMatch(req *http.Request) {
	fmt.Printf("DIFFER MATCH: %s %s", req.Method, req.URL.Path)
}

func (p PrintDiffReporter) ReportDiff(req *http.Request, diff string) {
	fmt.Printf("DIFFER DIFF: %s %s: %s", req.Method, req.URL.Path, diff)
}

func (p PrintDiffReporter) ReportError(req *http.Request, err error) {
	fmt.Printf("DIFFER ERROR: %s %s: %s", req.Method, req.URL.Path, err)
}
