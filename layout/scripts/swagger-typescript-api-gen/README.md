# typescript-api

### Adapter for PureAdmin

```js
import { Api, HttpClient } from "@/api/swagger/Api";
import { http } from "@/utils/http";
import type { AxiosInstance, AxiosResponse } from "axios";

interface PureHTTP {
    axiosInstance: AxiosInstance;
}

type UnwrapResponse<T> =
    T extends Promise<AxiosResponse<infer R>> ? Promise<R> : T;

type TransformFunction<T> = T extends (...args: infer Args) => infer Return
    ? (...args: Args) => UnwrapResponse<Return>
    : never;

type UnwrappedApiClient<T> = {
    [K in keyof Api<T>["api"]]: TransformFunction<Api<T>["api"][K]>;
};

type AdapterAPI<T> = UnwrappedApiClient<T>;

function createNewAPI(): AdapterAPI<unknown> {
    const client = new HttpClient<unknown>();
    client.instance = (http.constructor as unknown as PureHTTP).axiosInstance;
    client.instance.interceptors.response.use(
        resp => resp,
        (err: PureHttpError) => {
            if (!err.isCancelRequest) {
                if (err.response?.data) {
                    const {code, message} = err.response.data
                    as
                    GinxErrorResponse;
                    err.message = message;
                    err["errCode"] = code;
                }
            }
            return Promise.reject(err);
        }
    );
    const api = new Api<unknown>(client);
    return api.api as AdapterAPI<unknown>;
}

export const API = createNewAPI();

```