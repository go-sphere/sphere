package parser

import (
	"net/http"

	"google.golang.org/genproto/googleapis/api/annotations"
)

type HttpRule struct {
	Path         string
	Method       string
	HasBody      bool
	Body         string
	ResponseBody string
}

func ParseHttpRule(rule *annotations.HttpRule) *HttpRule {
	res := HttpRule{
		Method: http.MethodPost,
	}
	switch pattern := rule.Pattern.(type) {
	case *annotations.HttpRule_Get:
		res.Path = pattern.Get
		res.Method = http.MethodGet
	case *annotations.HttpRule_Put:
		res.Path = pattern.Put
		res.Method = http.MethodPut
	case *annotations.HttpRule_Post:
		res.Path = pattern.Post
		res.Method = http.MethodPost
	case *annotations.HttpRule_Delete:
		res.Path = pattern.Delete
		res.Method = http.MethodDelete
	case *annotations.HttpRule_Patch:
		res.Path = pattern.Patch
		res.Method = http.MethodPatch
	case *annotations.HttpRule_Custom:
		res.Path = pattern.Custom.Path
		res.Method = pattern.Custom.Kind
	default:
		res.Method = http.MethodPost
	}

	if rule.Body == "*" {
		res.HasBody = true
		res.Body = ""
	} else if rule.Body != "" {
		res.HasBody = true
		res.Body = rule.Body
	} else {
		res.HasBody = false
	}

	if rule.ResponseBody == "*" {
		res.ResponseBody = ""
	} else if rule.ResponseBody != "" {
		res.ResponseBody = rule.ResponseBody
	}
	return &res
}
