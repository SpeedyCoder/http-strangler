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
)

func newDiffHandler(oldHandler, newHandler http.Handler) http.Handler {
	// TODO: allow caller to provide these.
	differ := &diffingOrchestrator{
		differ:   CMPDiffer{},
		reporter: PrintDiffReporter{},
	}
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		oldHandlerReq, newHandlerReq, err := getRequestsToForward(req)
		if err != nil {
			http.Error(w, "failed to read payload", http.StatusInternalServerError)
			return
		}
		oldHandlerWriter, newHandlerWriter := httptest.NewRecorder(), httptest.NewRecorder()

		diffWG := &sync.WaitGroup{}
		diffWG.Add(2)
		// Asynchronously check for differences after both handlers are done.
		go differ.diffResponses(diffWG, req, oldHandlerWriter, newHandlerWriter)

		go func() {
			defer diffWG.Done()
			newHandler.ServeHTTP(newHandlerWriter, newHandlerReq)
		}()

		defer diffWG.Done()
		oldHandler.ServeHTTP(oldHandlerWriter, oldHandlerReq)
		copyResponse(oldHandlerWriter, w)
	})
}

func getRequestsToForward(req *http.Request) (*http.Request, *http.Request, error) {
	payload, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, nil, err
	}
	getBody := func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(payload)), nil
	}
	oldHandlerReq := req.Clone(req.Context())
	oldHandlerReq.Body = io.NopCloser(bytes.NewReader(payload))
	oldHandlerReq.GetBody = getBody

	newHandlerReq := req.Clone(context.Background())
	newHandlerReq.Body = io.NopCloser(bytes.NewReader(payload))
	oldHandlerReq.GetBody = getBody

	return oldHandlerReq, newHandlerReq, nil
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

type diffingOrchestrator struct {
	differ   Differ
	reporter DiffReporter
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
