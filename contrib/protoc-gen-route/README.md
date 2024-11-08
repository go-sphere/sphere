# protoc-gen-bot

This is a tool to generate `telegram bot` code from proto files.

## Installation

```shell
go install github.com/tbxark/sphere/contrib/protoc-gen-bot@latest
```

## Usage

Add `tbxark.options.options` to the rpc method, and set the key to `bot`, You can also add extra options to the method.

```proto

service CounterService {
  rpc Start(StartRequest) returns (StartResponse) {
    option (tbxark.options.options) = {
      key: "bot"
    };
  }
  rpc Counter(CounterRequest) returns (CounterResponse) {
    option (tbxark.options.options) = {
      key: "bot",
      extra: [
        {
          key: "command",
          value: "count",
        },
        {
          key: "callback_query",
          value: "count",
        }
      ]
    };
  }
  rpc Unknown(UnknownRequest) returns (UnknownResponse);
}
...
```

