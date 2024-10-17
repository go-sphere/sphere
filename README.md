# Sphere

**Sphere** is a multi-server application template that includes an API server, a dashboard server, and a bot server. It is designed to be a starting point for building a multi-server application.

This project uses minimal encapsulation, the simplest structure, and reduces code hierarchy to achieve rapid development while maintaining code readability and maintainability.

### Core Dependencies

- **Web Framework**: Gin
- **Dependency Injection**: Wire
- **ORM**: Ent

### Project Structure

```
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
```
### Usage

You can fork this project and modify the code in internal and cmd to implement your own business logic. Please do not modify the code in pkg. If necessary, please raise an issue or PR.

Alternatively, you can import this project in go mod and implement your own business logic in your project.

### License

**Sphere**  is released under the MIT license. See [LICENSE](LICENSE) for details.