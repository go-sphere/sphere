# Sphere

**Sphere** is a multi-server application template that includes an API server, a dashboard server, and a bot server. It is designed to be a starting point for building a multi-server application.

This project uses minimal encapsulation, the simplest structure, and reduces code hierarchy to achieve rapid development while maintaining code readability and maintainability.

You can define your api in the proto file and generate the code by running `make gen-proto`. You can also generate swagger docs by running `make gen-docs`.

### Usage
```
Usage: make <target>

Targets:
  init                Install all dependencies
  gen-proto           Generate proto files
  gen-docs            Generate swagger docs
  gen-ts              Generate typescript client
  generate            Generate code
  config              Generate config
  dash                Build dash
  build               Build binary
  build-linux-amd     Build linux amd64 binary
  build-linux-arm     Build linux arm64 binary
  build-all           Build all binary
  build-docker        Build docker image
  deploy              Deploy binary
  lint                Run linter
  help                Show this help message
```

### Core Dependencies

- **Web Framework**: Gin
- **Dependency Injection**: Wire
- **ORM**: Ent

### Project Structure

```
├── api                         # generated proto files
├── assets                      # embed assets
├── cmd                         # main entry
├── config                      # configuration
├── devops                      # devops configuration
├── docs                        # documentation generate by swag
├── internal                    # internal packages
│   ├── biz                     # business logic
│   ├── pkg                     # internal common packages
│   └── server                  # server
├── pkg                         # common packages
├── proto                       # proto files
```
### Usage

You can fork this project and modify the code in internal and cmd to implement your own business logic. Please do not modify the code in pkg. If necessary, please raise an issue or PR.

Alternatively, you can import this project in go mod and implement your own business logic in your project.

### License

**Sphere**  is released under the MIT license. See [LICENSE](LICENSE) for details.