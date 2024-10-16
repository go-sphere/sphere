# Sphere

**Sphere** is a multi-server application template that includes an API server, a dashboard server, and a bot server. It is designed to be a starting point for building a multi-server application.

### Project Structure
```
├── assets                      # embed assets
│   └── dash                    # embed dashboard frontend
├── cmd                         # main entry
│   ├── api                     # api server
│   ├── app                     # multi server, api, dashboard and bot
│   ├── bot                     # bot server
│   ├── config                  # configuration cli tool
│   ├── dash                    # dashboard server
│   └── storage                 # storage cli tool
├── configs                     # configuration
├── devops                      # devops configuration
├── docs                        # documentation generate by swag
├── internal                    # internal packages
│   ├── biz                     # business logic
│   │   ├── bot                 # bot-related business logic
│   │   ├── cron                # cron job-related business logic
│   │   └── task                # task-related business logic
│   ├── pkg                     # common packages
│   │   ├── consts              # constants
│   │   ├── dao                 # data access objects
│   │   ├── database            # database-related packages
│   │   ├── render              # model rendering utilities
│   │   └── scache              # cache-related packages
│   └── server                  # server
│       ├── api                 # API server implementation
│       └── dash                # Dashboard server implementation
├── pkg                         # common packages
│   ├── cache                   # cache
│   │   ├── memory              # in-memory cache implementation
│   │   └── redis               # Redis cache implementation
│   ├── database                # database
│   │   └── sqlite              # SQLite database implementation
│   ├── log                     # log
│   │   └── logfields           # log field definitions
│   ├── search                  # search
│   │   └── meilisearch         # Meilisearch implementation
│   ├── storage                 # storage
│   │   ├── qiniu               # Qiniu storage implementation
│   │   └── s3                  # S3 storage implementation
│   ├── telegram                # Telegram-related utilities
│   ├── utils                   # utility functions
│   │   ├── boot                # boot-related utilities
│   │   ├── encrypt             # encryption utilities
│   │   ├── idgenerator         # ID generation utilities
│   │   ├── request             # request-related utilities
│   │   └── safe                # safety-related utilities
│   ├── web                     # web-related packages
│   │   ├── auth                # authentication packages
│   │   │   ├── jwtauth         # JWT authentication
│   │   │   ├── parser          # authentication parser
│   │   │   └── tmaauth         # TMA authentication
│   │   ├── docs                # web documentation
│   │   └── middleware          # web middleware
│   │       ├── auth            # authentication middleware
│   │       ├── logger          # logging middleware
│   │       └── ratelimiter     # rate limiting middleware
│   └── wechat                  # WeChat-related utilities
```
### Usage

| Executable | Description                                    |
|------------|------------------------------------------------|
| `api`      | Start the API server                           |
| `dash`     | Start the dashboard server                     |
| `bot`      | Start the bot server                           |
| `app`      | Start the multi server, api, dashboard and bot |
| `config`   | Configuration cli tool                         |
| `storage`  | Storage cli tool                               |

You can fork this project and modify the code in internal and cmd to implement your own business logic. Please do not modify the code in pkg. If necessary, please raise an issue or PR.

Alternatively, you can import this project in go mod and implement your own business logic in your project.

### License

**Sphere**  is released under the MIT license. See [LICENSE](LICENSE) for details.