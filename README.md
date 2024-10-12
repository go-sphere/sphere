# go-base-api


### Usage

| Executable  | Description                                    |
|-------------|------------------------------------------------|
| `api`       | Start the API server                           |
| `dashboard` | Start the dashboard server                     |
| `bot`       | Start the bot server                           |
| `app`       | Start the multi server, api, dashboard and bot |
| `config`    | Configuration cli tool                         |
| `storage`   | Storage cli tool                               |

### Generate typescript client

```
npx swagger-typescript-api -p ./swagger.json -o ./src/api  --modular 
```