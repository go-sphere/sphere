import {generateApi} from "swagger-typescript-api";
import {fileURLToPath} from "url";
import {dirname, join} from "path";

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

console.log(__dirname)

await generateApi({
    input: join(__dirname, "../api/API_swagger.json"),
    output: join(__dirname, "../api/typescript"),
    httpClientType: "fetch",
    extractRequestBody: true,
    extractResponseBody: true,
    defaultResponseAsSuccess: true,
});

await generateApi({
    input: join(__dirname, "../dash/Dash_swagger.json"),
    output: join(__dirname, "../dash/typescript"),
    httpClientType: "axios",
    extractRequestBody: true,
    extractResponseBody: true,
    defaultResponseAsSuccess: true,
});