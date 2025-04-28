package database

//go:generate go tool ent generate --feature sql/modifier,sql/execquery,sql/upsert,sql/lock --target ./ent ./schema
