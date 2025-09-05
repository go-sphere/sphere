package ginx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/go-viper/mapstructure/v2"
	"github.com/stretchr/testify/assert"
)

func ptr[T any](v T) *T {
	return &v
}

func TestShouldUniverseBind(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()

	type Request struct {
		state         any      `protogen:"open.v1"` //nolint
		FieldTest1    string   `protobuf:"bytes,1,opt,name=field_test1,json=fieldTest1,proto3" json:"field_test1,omitempty"`
		FieldTest2    int64    `protobuf:"varint,2,opt,name=field_test2,json=fieldTest2,proto3" json:"field_test2,omitempty"`
		PathTest1     string   `protobuf:"bytes,3,opt,name=path_test1,json=pathTest1,proto3" json:"path_test1,omitempty"`
		PathTest2     int64    `protobuf:"varint,4,opt,name=path_test2,json=pathTest2,proto3" json:"-" uri:"path_test2"`
		QueryTest1    string   `protobuf:"bytes,5,opt,name=query_test1,json=queryTest1,proto3" json:"query_test1,omitempty"`
		QueryTest2    *int64   `protobuf:"varint,6,opt,name=query_test2,json=queryTest2,proto3" json:"-" form:"query_test2,omitempty"`
		QueryArray    []string `protobuf:"bytes,7,rep,name=query_array,json=queryArray,proto3" json:"query_array,omitempty" form:"query_array,omitempty"`
		unknownFields any      //nolint
		sizeCache     any      //nolint
	}

	type Response struct {
		FieldTest1 string   `json:"field_test1,omitempty"`
		FieldTest2 int64    `json:"field_test2,omitempty"`
		PathTest1  string   `json:"path_test1,omitempty"`
		PathTest2  int64    `json:"path_test2,omitempty"`
		QueryTest1 string   `json:"query_test1,omitempty"`
		QueryTest2 *int64   `json:"query_test2,omitempty"`
		QueryArray []string `json:"query_array,omitempty"`
	}

	params := &Request{
		FieldTest1: "field",
		FieldTest2: 123,
		PathTest1:  "path",
		PathTest2:  456,
		QueryTest1: "query",
		QueryTest2: ptr[int64](789),
		QueryArray: []string{"value1", "value2"},
	}

	paramsRaw, err := json.Marshal(map[string]any{
		"field_test1": params.FieldTest1,
		"field_test2": params.FieldTest2,
	})
	if err != nil {
		t.Fatalf("Failed to marshal params: %v", err)
	}
	router.POST("/api/test/:path_test1/second/:path_test2", func(c *gin.Context) {
		var input Request
		if ShouldUniverseBind(c, &input, true, true, true) != nil {
			c.AbortWithStatus(400)
			return
		}
		var output Response
		if mapstructure.Decode(input, &output) != nil {
			c.AbortWithStatus(400)
			return
		}
		c.JSON(http.StatusOK, output)
	})

	writer := httptest.NewRecorder()
	query := url.Values{}
	query.Add("query_test1", params.QueryTest1)
	query.Add("query_test2", fmt.Sprintf("%d", *params.QueryTest2))
	for _, v := range params.QueryArray {
		query.Add("query_array", v)
	}

	uri := fmt.Sprintf("/api/test/%s/second/%d?%s", params.PathTest1, params.PathTest2, query.Encode())
	req, err := http.NewRequest("POST", uri, bytes.NewReader(paramsRaw))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	router.ServeHTTP(writer, req)

	var resp Response
	err = json.Unmarshal(writer.Body.Bytes(), &resp)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	assert.Equal(t, params.FieldTest1, resp.FieldTest1, "Expected FieldTest1 to match")
	assert.Equal(t, params.FieldTest2, resp.FieldTest2, "Expected FieldTest2 to match")
	assert.Equal(t, params.PathTest1, resp.PathTest1, "Expected PathTest1 to match")
	assert.Equal(t, params.PathTest2, resp.PathTest2, "Expected PathTest2 to match")
	assert.Equal(t, params.QueryTest1, resp.QueryTest1, "Expected QueryTest1 to match")
	assert.Equal(t, params.QueryArray, resp.QueryArray, "Expected QueryArray to match")

	if params.QueryTest2 != nil {
		assert.Equal(t, *params.QueryTest2, *resp.QueryTest2, "Expected QueryTest2 to match")
	} else {
		assert.Nil(t, resp.QueryTest2, "Expected QueryTest2 to be nil")
	}
}

func TestShouldUniverseBindForm(t *testing.T) {
	type Request struct {
		Foo string `form:"foo"`
	}
	type Response struct {
		Foo string `json:"foo"`
	}
	router := gin.New()
	router.POST("/test", func(c *gin.Context) {
		var req Request
		if err := ShouldUniverseBindForm(c, &req); err != nil {
			c.AbortWithStatus(400)
			return
		}
		c.JSON(200, Response{Foo: req.Foo})
	})
	{
		form := url.Values{}
		form.Add("foo", "bar")
		req, _ := http.NewRequest("POST", "/test", strings.NewReader(form.Encode()))
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code)
		var resp Response
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.Equal(t, "bar", resp.Foo)
	}
	{
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		_ = writer.WriteField("foo", "bar")
		_ = writer.Close()
		req, _ := http.NewRequest("POST", "/test", body)
		req.Header.Add("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code)
		var resp Response
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.Equal(t, "bar", resp.Foo)
	}
}
