# Documentation for Sablier

<a name="documentation-for-api-endpoints"></a>
## Documentation for API Endpoints

All URIs are relative to *http://localhost:10000*

| Class | Method | HTTP request | Description |
|------------ | ------------- | ------------- | -------------|
| *ScaleApi* | [**scaleBlocking**](Apis/ScaleApi.md#scaleblocking) | **GET** /api/strategies/blocking | Hangs the request until the services are ready |
*ScaleApi* | [**scaleDynamic**](Apis/ScaleApi.md#scaledynamic) | **GET** /api/strategies/dynamic | The waiting page for the given services |
| *ThemeApi* | [**getTheme**](Apis/ThemeApi.md#gettheme) | **GET** /api/strategies/dynamoc/themes |  |


<a name="documentation-for-models"></a>
## Documentation for Models

 - [instance](./Models/instance.md)
 - [session](./Models/session.md)
 - [status](./Models/status.md)
 - [themes](./Models/themes.md)


<a name="documentation-for-authorization"></a>
## Documentation for Authorization

All endpoints do not require authorization.

## API

To run the following examples you can create two containers:

- `docker create --name nginx nginx`
- `docker create --name apache httpd`

### GET `/api/strategies/dynamic`

**Description**: The `/api/strategies/dynamic` endpoint allows you to request a waiting page for multiple instances

| Parameter                        | Value                                                                | Description                                                                                                     |
| -------------------------------- | -------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------- |
| `names`                          | array of string                                                      | The instances to be started (cannot be used with `group` parameter)                                             |
| `group`                          | string                                                               | The instance group to be started (using `sablier.group=mygroup` labels) (cannot be used with `names` parameter) |
| `session_duration`               | duration [time.ParseDuration](https://pkg.go.dev/time#ParseDuration) | The session duration for all services, which will reset at each subsequent calls                                |
| `show_details` *(optional)*      | bool                                                                 | The details about instances                                                                                     |
| `display_name` *(optional)*      | string                                                               | The display name                                                                                                |
| `theme` *(optional)*             | string                                                               | The theme to use                                                                                                |
| `refresh_frequency` *(optional)* | duration [time.ParseDuration](https://pkg.go.dev/time#ParseDuration) | The refresh frequency for the loading page                                                                      |

Go to http://localhost:10000/api/strategies/dynamic?names=nginx&names=apache&session_duration=5m&show_details=true&display_name=example&theme=hacker-terminal&refresh_frequency=10s and you should see

A special header `X-Sablier-Session-Status` is returned and will have the value `ready` if all instances are ready. Or else `not-ready`.

![API Dynamic Prompt image](docs/img/api-dynamic.png)

### GET `/api/strategies/blocking`

**Description**: The `/api/strategies/blocking` endpoint allows you to wait until the instances are ready

| Parameter              | Value                                                                | Description                                                                                                     |
| ---------------------- | -------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------- |
| `names`                | array of string                                                      | The instances to be started (cannot be used with `group` parameter)                                             |
| `group`                | string                                                               | The instance group to be started (using `sablier.group=mygroup` labels) (cannot be used with `names` parameter) |
| `session_duration`     | duration [time.ParseDuration](https://pkg.go.dev/time#ParseDuration) | The session duration for all services, which will reset at each subsequent calls                                |
| `timeout` *(optional)* | duration [time.ParseDuration](https://pkg.go.dev/time#ParseDuration) | The maximum time to wait for instances to be ready                                                              |

A special header `X-Sablier-Session-Status` is returned and will have the value `ready` if all instances are ready. Or else `not-ready`.

**Curl example**
```bash
curl -X GET -v "http://localhost:10000/api/strategies/blocking?names=nginx&names=apache&session_duration=5m&timeout=5s"
*   Trying 127.0.0.1:10000...
* Connected to localhost (127.0.0.1) port 10000 (#0)
> GET /api/strategies/blocking?names=nginx&names=apache&session_duration=5m&timeout=30s HTTP/1.1
> Host: localhost:10000
> User-Agent: curl/7.74.0
> Accept: */*
> 
* Mark bundle as not supporting multiuse
< HTTP/1.1 200 OK
< Content-Type: application/json; charset=utf-8
< X-Sablier-Session-Status: ready
< Date: Mon, 14 Nov 2022 19:20:50 GMT
< Content-Length: 245
< 
{"session":
  {"instances":
    [
      {"instance":{"name":"nginx","currentReplicas":1,"desiredReplicas":1,"status":"ready"},"error":null},
      {"instance":{"name":"apache","currentReplicas":1,"desiredReplicas":1,"status":"ready"},"error":null}
    ],
    "status":"ready"
  }
}
```