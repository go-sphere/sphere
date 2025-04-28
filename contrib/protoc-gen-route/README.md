# protoc-gen-route

This is a protoc plugin to generate route code from proto files

## Installation

```shell
go install github.com/TBXark/sphere/contrib/protoc-gen-route@latest
```

## Example

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

And then run the command, add following to your `buf.gen.yaml` file

```yaml
  - local: protoc-gen-route
    out: api
    opt:
      - paths=source_relative
      - options_key=bot # The key of the options
      - gen_file_suffix=_bot.pb.go # The suffix of the generated file
      - request_model=github.com/TBXark/sphere/telegram;Update
      - response_model=github.com/TBXark/sphere/telegram;Message
      - extra_data_model=github.com/TBXark/sphere/telegram;MethodExtraData
      - extra_data_constructor=github.com/TBXark/sphere/telegram;NewMethodExtraData
```
