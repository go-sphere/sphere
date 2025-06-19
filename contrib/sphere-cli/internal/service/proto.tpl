syntax = "proto3";

package {{.PackageName}};

import "google/api/annotations.proto";
import "entpb/entpb.proto";


service {{.ServiceName}}Service {
  rpc {{.ServiceName}}List({{.ServiceName}}ListRequest) returns ({{.ServiceName}}ListResponse) {
    option (google.api.http) = {
      get: "/api/{{.RouteName}}/list"
    };
  }
  rpc {{.ServiceName}}Create({{.ServiceName}}CreateRequest) returns ({{.ServiceName}}CreateResponse) {
    option (google.api.http) = {
      post: "/api/{{.RouteName}}/create"
      body: "*"
    };
  }
  rpc {{.ServiceName}}Update({{.ServiceName}}UpdateRequest) returns ({{.ServiceName}}UpdateResponse) {
    option (google.api.http) = {
      post: "/api/{{.RouteName}}/update"
      body: "*"
    };
  }
  rpc {{.ServiceName}}Detail({{.ServiceName}}DetailRequest) returns ({{.ServiceName}}DetailResponse) {
    option (google.api.http) = {
      get: "/api/{{.RouteName}}/detail/{id}"
    };
  }
  rpc {{.ServiceName}}Delete({{.ServiceName}}DeleteRequest) returns ({{.ServiceName}}DeleteResponse) {
    option (google.api.http) = {
      delete: "/api/{{.RouteName}}/delete/{id}"
    };
  }
}

message {{.ServiceName}}ListRequest {
  int64 page = 1 [
    (buf.validate.field).int64.gte = 0
  ];
  int64 page_size = 2 [
    (buf.validate.field).int64.gte = 0
  ];
}

message {{.ServiceName}}ListResponse {
  repeated entpb.{{.ServiceName}} {{.EntityName}}s = 1;
  int64 total_size = 2;
  int64 total_page = 3;
}

message {{.ServiceName}}CreateRequest {
  entpb.{{.ServiceName}} {{.EntityName}} = 1;
}

message {{.ServiceName}}CreateResponse {
  entpb.{{.ServiceName}} {{.EntityName}} = 1;
}

message {{.ServiceName}}UpdateRequest {
  entpb.{{.ServiceName}} {{.EntityName}} = 1;
}

message {{.ServiceName}}UpdateResponse {
  entpb.{{.ServiceName}} {{.EntityName}} = 1;
}

message {{.ServiceName}}DetailRequest {
  int64 id = 1; // @gotags: json:"-"` uri:"id"`
}

message {{.ServiceName}}DetailResponse {
  entpb.{{.ServiceName}} {{.EntityName}} = 1;
}

message {{.ServiceName}}DeleteRequest {
  int64 id = 1; // @gotags: json:"-"` uri:"id"`
}

message {{.ServiceName}}DeleteResponse {
}