package ginx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestShouldBind(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.Default()
	type Params struct {
		FieldTest1 string `protobuf:"bytes,1,opt,name=field_test1,json=fieldTest1,proto3" json:"field_test1,omitempty"`
		FieldTest2 int64  `protobuf:"varint,2,opt,name=field_test2,json=fieldTest2,proto3" json:"field_test2,omitempty"`
		PathTest1  string `protobuf:"bytes,3,opt,name=path_test1,json=pathTest1,proto3" json:"path_test1,omitempty"`
		PathTest2  int64  `protobuf:"varint,4,opt,name=path_test2,json=pathTest2,proto3" json:"-" uri:"path_test2"`
		QueryTest1 string `protobuf:"bytes,5,opt,name=query_test1,json=queryTest1,proto3" json:"query_test1,omitempty"`
		QueryTest2 int64  `protobuf:"varint,6,opt,name=query_test2,json=queryTest2,proto3" json:"-" form:"query_test2,omitempty"`
	}
	params := &Params{
		FieldTest1: "field",
		FieldTest2: 123,
		PathTest1:  "path",
		PathTest2:  456,
		QueryTest1: "query",
		QueryTest2: 789,
	}
	paramsRaw, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Failed to marshal params: %v", err)
	}

	router.GET("/api/test/:path_test1/second/:path_test2", func(c *gin.Context) {
		var obj Params
		if ShouldBind(c, &obj, true, true, true) != nil {
			c.AbortWithStatus(400)
		} else {
			assert.Equal(t, params.FieldTest1, obj.FieldTest1)
			assert.Equal(t, params.FieldTest2, obj.FieldTest2)
			assert.Equal(t, params.PathTest1, obj.PathTest1)
			assert.Equal(t, params.PathTest2, obj.PathTest2)
			assert.Equal(t, params.QueryTest1, obj.QueryTest1)
			c.AbortWithStatus(200)
		}
	})

	w := httptest.NewRecorder()
	query := url.Values{}
	query.Add("query_test1", params.QueryTest1)
	query.Add("query_test2", fmt.Sprintf("%d", params.QueryTest2))
	uri := fmt.Sprintf("/api/test/%s/second/%d?%s", params.PathTest1, params.PathTest2, query.Encode())
	req, _ := http.NewRequest("GET", uri, bytes.NewReader(paramsRaw))
	router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
}
