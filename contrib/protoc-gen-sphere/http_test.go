package main

import (
	"testing"
)

func Test_buildGinRoutePath(t *testing.T) {
	examples := []string{
		"/v1/{name=messages/*}",
		"/v1/{name=messages/*/sub/*}",
		"/v1/{name}",
	}
	expected := []string{
		"/v1/*name",
		"/v1/*name",
		"/v1/:name",
	}
	mapToString := func(m map[string]*string) string {
		s := "{"
		for k, v := range m {
			if v == nil {
				s += k + ": nil, "
				continue
			}
			s += k + ": " + *v + ", "
		}
		s += "}"
		return s
	}
	for i, path := range examples {
		vars, _ := buildPathVars(path)
		for v, s := range vars {
			if s != nil {
				path = replacePath(v, *s, path)
			}
		}
		ginPath := buildGinRoutePath(path)
		t.Logf("path: %s, vars: %s", path, mapToString(vars))
		t.Logf("ginPath: %s", ginPath)
		t.Logf("swaggerPath: %s", buildSwaggerPath(path))
		if ginPath != expected[i] {
			t.Errorf("expected: %s, got: %s", expected[i], ginPath)
		}
	}
}

func Test_findQueryParam(t *testing.T) {
	name := findQueryParam("// @sphere:form=\"query_test2\"", "demo")
	t.Logf("name: %s", name)
}
