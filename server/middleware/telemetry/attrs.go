package telemetry

import (
	"net/http"
	"strconv"

	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
)

// Every semconv symbol the package uses lives in this file, so moving to a
// later semconv version is one diff.

const (
	keyRoute          = "http.route"
	keyStatusCode     = "http.response.status_code"
	keyMethod         = "http.request.method"
	keyMethodOriginal = "http.request.method_original"
	keyErrorType      = "error.type"
)

// normalizeMethod maps the request method to one of the standard HTTP methods,
// or "_OTHER" with the original spelling returned separately. The method is
// client controlled, so passing it through would make the label unbounded.
func normalizeMethod(m string) (method, original string) {
	switch m {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut,
		http.MethodDelete, http.MethodConnect, http.MethodOptions,
		http.MethodTrace, http.MethodPatch:
		return m, ""
	}
	return "_OTHER", m
}

func methodAttr(method string) attribute.KeyValue {
	return semconv.HTTPRequestMethodKey.String(method)
}

func methodOriginalAttr(m string) attribute.KeyValue {
	return semconv.HTTPRequestMethodOriginal(m)
}

func routeAttr(route string) attribute.KeyValue { return semconv.HTTPRoute(route) }

func statusAttr(status int) attribute.KeyValue { return semconv.HTTPResponseStatusCode(status) }

func pathAttr(path string) attribute.KeyValue { return semconv.URLPath(path) }

func schemeAttr(s string) attribute.KeyValue { return semconv.URLScheme(s) }

func userAgentAttr(ua string) attribute.KeyValue { return semconv.UserAgentOriginal(ua) }

func errorTypeAttr(status int, panicked bool) (attribute.KeyValue, bool) {
	switch {
	case panicked:
		return semconv.ErrorTypeKey.String("panic"), true
	case status >= http.StatusInternalServerError:
		return semconv.ErrorTypeKey.String(strconv.Itoa(status)), true
	}
	return attribute.KeyValue{}, false
}
