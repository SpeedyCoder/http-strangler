package httpstrangler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"
)

type diffingOrchestrator struct {
	defaultHandler     http.Handler
	alternativeHandler http.Handler
	differ             Differ
	reporter           DiffReporter
	alternativeTimeout time.Duration
	alternativeContext func(context.Context) context.Context
}

func (d *diffingOrchestrator) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	defaultReq, alternativeReq, alternativeCancel, err := d.getRequestsToForward(req)
	if err != nil {
		http.Error(w, "failed to read payload", http.StatusInternalServerError)
		return
	}
	defaultWriter, alternativeWriter := httptest.NewRecorder(), httptest.NewRecorder()

	diffWG := &sync.WaitGroup{}
	diffWG.Add(2)
	// Asynchronously check for differences after both handlers are done.
	go d.diffResponses(diffWG, req, defaultWriter, alternativeWriter)

	go func() {
		defer diffWG.Done()
		defer alternativeCancel()
		d.alternativeHandler.ServeHTTP(alternativeWriter, alternativeReq)
	}()

	defer diffWG.Done()
	d.defaultHandler.ServeHTTP(defaultWriter, defaultReq)
	copyResponse(defaultWriter, w)
}

func (d *diffingOrchestrator) getRequestsToForward(req *http.Request) (*http.Request, *http.Request, func(), error) {
	payload, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, nil, nil, err
	}
	getBody := func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(payload)), nil
	}
	defaultReq := req.Clone(req.Context())
	defaultReq.Body = io.NopCloser(bytes.NewReader(payload))
	defaultReq.GetBody = getBody

	alternativeCtx, alternativeCancel := context.WithTimeout(d.alternativeContext(req.Context()), d.alternativeTimeout)
	alternativeReq := req.Clone(alternativeCtx)
	alternativeReq.Body = io.NopCloser(bytes.NewReader(payload))
	alternativeReq.GetBody = getBody

	return defaultReq, alternativeReq, alternativeCancel, nil
}

func (d *diffingOrchestrator) diffResponses(wg *sync.WaitGroup, req *http.Request, defaultResp, alternativeResp *httptest.ResponseRecorder) {
	wg.Wait() // Wait for both requests to finish.

	var defaultJSON, alternativeJSON any

	if err := json.Unmarshal(defaultResp.Body.Bytes(), &defaultJSON); err != nil {
		d.reporter.ReportError(req, fmt.Errorf("failed to unmarshal default json: %w", err))
	}
	if err := json.Unmarshal(alternativeResp.Body.Bytes(), &alternativeJSON); err != nil {
		d.reporter.ReportError(req, fmt.Errorf("failed to unmarshal alternative json: %w", err))
	}
	defaultResponse := &Response{
		Headers:    defaultResp.Header(),
		StatusCode: defaultResp.Code,
		Body:       defaultJSON,
	}
	alternativeResponse := &Response{
		Headers:    alternativeResp.Header(),
		StatusCode: alternativeResp.Code,
		Body:       alternativeJSON,
	}
	if diff := d.differ.Diff(defaultResponse, alternativeResponse); diff != "" {
		d.reporter.ReportDiff(req, diff)
	} else {
		d.reporter.ReportMatch(req)
	}
}

func copyResponse(recorder *httptest.ResponseRecorder, w http.ResponseWriter) {
	for name, values := range recorder.Header() {
		for _, val := range values {
			w.Header().Add(name, val)
		}
	}
	w.WriteHeader(recorder.Code)
	_, _ = w.Write(recorder.Body.Bytes())
}
