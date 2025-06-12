package wechat

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestJsCode2SessionResponse(t *testing.T) {
	resp := JsCode2SessionResponse{
		ErrResponse: ErrResponse{
			ErrCode: 12,
			ErrMsg:  "34",
		},
		OpenID:     "56",
		SessionKey: "78",
		UnionID:    "90",
	}
	indent, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal JsCode2SessionResponse: %v", err)
	}
	t.Logf("JsCode2SessionResponse: %s", indent)
	var result JsCode2SessionResponse
	err = json.Unmarshal(indent, &result)
	if err != nil {
		t.Fatalf("failed to unmarshal JsCode2SessionResponse: %v", err)
	}
	if !reflect.DeepEqual(resp, result) {
		t.Errorf("unmarshaled JsCode2SessionResponse does not match original: got %v, want %v", result, resp)
	}
}
