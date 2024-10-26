# ent-gen-proto

This is a tool to generate proto files from ent schema. 

Different from the [entproto](https://github.com/ent/contrib/tree/master/entproto) tool, `entproto` needs to add annotations to the schema, and `ent-gen-proto` does not need to add any annotations to the schema. And `entproto` does not support the `optional` field, and `ent-gen-proto` supports the `optional` field.

## Installation

```shell
go get ./..
go install .
```

Due to the use of `replace` in go mod, it is not possible to directly use `go install`. You need to use `go get github.com/tbxark/sphere/contrib/ent-gen-proto@latest` to install.