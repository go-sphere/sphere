# ent-gen-proto

This is a tool to generate proto files from ent schema. 

Different from the [entproto](https://github.com/ent/contrib/tree/master/entproto) tool, `entproto` needs to add annotations to the schema, and `ent-gen-proto` does not need to add any annotations to the schema. And `entproto` does not support the `optional` field, I submitted the pr hopefully the merger will happen soon

## Installation

```shell
go install github.com/TBXark/sphere/contrib/ent-gen-proto@latest
```
