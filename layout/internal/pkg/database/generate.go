package database

//go:generate go run -mod=mod entgo.io/ent/cmd/ent generate --feature sql/modifier,sql/execquery,sql/upsert,sql/lock --target ./ent ./schema
