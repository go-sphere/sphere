package main

import "github.com/tbxark/go-base-api/cmd"

// @securityDefinitions.apikey	ApiKeyAuth
// @in							header
// @name						Authorization
// @description				    JWT token
func main() {
	cmd.Execute()
}
