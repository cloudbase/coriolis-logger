# Coriolis logger

Coriolis logger is a central logging daemon created to integrate in a Coriolis deployment. It is a small Syslog server, with support for multiple writers, both persistent (InfluxDB) and transient (web sockets, standard output). The goal of this project is to allow easy streaming of logs to any web sockets enabled destination (WebUI, CLI, etc), as well as allow users of Coriolis to easily download logs that are aggregated by this service.

Conceivably this app can be used with any application that logs to Syslog, and data-store packages can be written for it to allow sending logs to any backend. Currently we support the following writers and data stores:

  * Data stores:
    * InfluxDB
  * Writers:
    * Web Sockets
    * Standard out (testing purposes mostly)

## Building the binary

Clone the repo:

```bash
git clone https://github.com/cloudbase/coriolis-logger
```

Build the binary:

```bash
cd coriolis-logger
go install ./...
```

## Configuration

Coriolis logger uses a simple ```toml``` file as a config:

```toml
[apiserver]
bind = "0.0.0.0"
port = 9998
use_tls = false

# Authentication middleware to use. Available options are:
#  * keystone
#  * none
# coriolis-logger will refuse to start if this option is
# missing. To disable authentication, you must explicitly
# set this option to "none"
auth_middleware = "keystone"

    [apiserver.keystone_auth]
    # The keystone auth URI
    auth_uri = "http://127.0.0.1:5000/v3"

    # API server TLS config
    [apiserver.tls]
    crt = "/tmp/certificate.pem"
    key = "/tmp/key.pem"
    cacert = "/tmp/ca-cert.pem"

[syslog]
# Possible values: unixgram, tcp, udp
listener = "unixgram"

# possible values:
#   for unixgram: /path/to/socket
#   for tcp/udp IP:port pair: 0.0.0.0:5144 
# address = "/tmp/coriolis-logger/syslog"
address = "/tmp/coriolis-logging.sock"

# Log format
# possible values:
#   rfc3164
#   rfc5424
#   rfc6587
#   automatic
format = "automatic"

# Whether to dump logs to stdout or not
# this should only be enabled for testng purposes
log_to_stdout = false

# storage backend for logs. Available options are:
#   * influxdb
datastore = "influxdb"

    [syslog.influxdb]
    url = "http://127.0.0.1:8086"
    # If influxDB auth is enabled, use this username
    username = "coriolis"
    # A super secret password. Obviously needs changing :-)
    password = "Passw0rd"
    # Logs database
    database = "coriolis"
    # duration in seconds after which this datastore
    # flushes writes to the database
    write_interval = 5
    # Verify server enables mutual TLS authentication
    verify_server = false
    # Client TLS certificates
    # cacert = "/tmp/ca.pem"
    # client_crt = "/tmp/client-crt.pem"
    # client_key = "/tmp/client-key.pem"

    # The retention period for logs in days. Logs older than
    # this, will be deleted. If missing, this option default
    # to 3 days. This setting will be moved in the future
    # under the [syslog] section, when we will support multiple
    # datastores.
    log_retention_period = 3
```

## Usage

Depending on the authentication middleware used, additional headers may need to be set.

Generally available query parameters:

|    Name    | Type    | Optional | Description                                                                  |
| ---------- | ------- | -------- | ---------------------------------------------------------------------------- |
| auth_type  | string  |   true   | Authentication token type. Supported authentication methods are: keystone. This option must match the authentication middleware enabled in the config for coriolis-logger |
| auth_token | string  |   true   | Authentication token/credentials for the selected auth_type. |


### List logs

```
GET /api/v1/logs/
```

Example:

```bash
$ curl -s -H "X-Auth-Token: <token_goes_here>" -X GET http://127.0.0.1:9998/api/v1/logs/ | jq
{
  "logs": [
    {
      "log_name": "coriolis-api"
    },
    {
      "log_name": "coriolis-conductor"
    },
    {
      "log_name": "coriolis-dbsync"
    },
    {
      "log_name": "coriolis-replica-cron"
    },
    {
      "log_name": "coriolis-worker"
    }
  ]
}
```

Alternatively, authentication method and authentication info can be specified in the GET query args:

```bash
curl -s -X GET "http://127.0.0.1:9998/api/v1/logs/?auth_type=keystone&auth_token=super_secret_token" | jq
{
  "logs": [
    {
      "log_name": "coriolis-api"
    },
    {
      "log_name": "coriolis-conductor"
    },
    {
      "log_name": "coriolis-dbsync"
    },
    {
      "log_name": "coriolis-replica-cron"
    },
    {
      "log_name": "coriolis-worker"
    }
  ]
}
```

### Download logs

```
GET /api/v1/logs/{log_name}/
```

Query parameters:

|      Name       | Type | Optional | Description                                                                  |
| --------------- | ---- | -------- | ---------------------------------------------------------------------------- |
|   start_date    | int  |   true   | Unix timestamp indicating the start date from which we want to download logs |
|    end_date     | int  |   true   | Unix timestamp indicating the end date to which we want to download logs     |
| disable_chunked | bool |   true   | If true, coriolis-logger will attempt to disable chunked transfer.           |

### Stream logs using web sockets

```
GET /api/v1/ws/
```

Query parameters:

|    Name    |  Type   | Optional | Description                                                                               |
| ---------- | ------- | -------- | ----------------------------------------------------------------------------------------- |
| severity   |   int   |   true   | Severity level. Values range from 0 to 7. See https://tools.ietf.org/html/rfc5424#page-11 |
| app_name   |  string |   true   | The name of the log we wish to stream. See the "list" section.                            |


Example:

```python
#!/usr/bin/env python3

import websockets
import asyncio
import json

token = "super secret token"

async def hello():
    uri = "ws://127.0.0.1:9998/api/v1/ws/?severity=7&app_name=coriolis-worker"
    async with websockets.connect(uri, extra_headers={"X-Auth-Token": token}) as websocket:
        while True:
            msg = await websocket.recv()
            asDict = json.loads(msg)
            print(asDict["message"])

try:
    asyncio.get_event_loop().run_until_complete(hello())
except KeyboardInterrupt:
    print("stopped")

```

## Using with docker

If coriolis-logger is configured to listen on ```/tmp/coriolis-logger.sock```, to use it with a docker container, you simply have to mount the socket file as ```/dev/log``` inside the container.

```bash
$ docker run --rm -it -v /tmp/coriolis-logging.sock:/dev/log ubuntu:latest bash
root@415b368958bc:/# ls -l /dev/log
srwxrwxr-x 1 1000 1000 0 Oct 21 23:11 /dev/log
root@415b368958bc:/# logger --rfc5424 "this is a test"
```

If you are listening on web sockets using the above example, you should now see the log message on your screen. 
