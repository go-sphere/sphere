# protoc-gen-bot

This is a tool to generate `telegram bot` code from proto files.

## Installation

```shell
go install github.com/tbxark/sphere/contrib/protoc-gen-bot@latest
```

## Usage

Add `// @bot` to the comment of the service, and the bot will generate the code.

```proto
syntax = "proto3";

package helloworld;

// @bot
service Greeter {
  rpc SayHello (HelloRequest) returns (HelloReply) {}
}

...
```