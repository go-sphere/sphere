package shared

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	sharedv1 "github.com/TBXark/sphere/layout/api/shared/v1"
	"github.com/TBXark/sphere/server/ginx"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestService_RunTest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.Default()
	sharedv1.RegisterTestServiceHTTPServer(router, &Service{})

	req := sharedv1.RunTestRequest{
		FieldTest1: "test1",
		FieldTest2: 2,
		PathTest1:  "path1",
		PathTest2:  200,
		QueryTest1: "query1",
		QueryTest2: 2000,
	}

	query := url.Values{}
	query.Add("query_test1", req.QueryTest1)
	query.Add("query_test2", fmt.Sprintf("%d", req.QueryTest2))

	body, _ := json.Marshal(&req)

	uri := fmt.Sprintf("/api/test/%s/second/%d?%s", req.PathTest1, req.PathTest2, query.Encode())
	request, err := http.NewRequest("POST", uri, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	var resp ginx.DataResponse[sharedv1.RunTestResponse]
	err = json.Unmarshal(recorder.Body.Bytes(), &resp)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	assert.Equal(t, resp.Data.FieldTest1, req.FieldTest1)
	assert.Equal(t, resp.Data.FieldTest2, req.FieldTest2)
	assert.Equal(t, resp.Data.PathTest1, req.PathTest1)
	assert.Equal(t, resp.Data.PathTest2, req.PathTest2)
	assert.Equal(t, resp.Data.QueryTest1, req.QueryTest1)
	assert.Equal(t, resp.Data.QueryTest2, req.QueryTest2)
	assert.Equal(t, http.StatusOK, recorder.Code, "Expected status code 200, got %d", recorder.Code)
}
