#🐸 Frogpond

Frogpond is a simple, peer-to-peer, CRDT, key value store and host discovery service.  I use it on all my home computers to find each other and share configuration and a small amount of data.  It is also available as a library for other applications, e.g. [clusterF](https://github.com/donomii/clusterf)

Frogpond is not secure.  It is not encrypted, and it does not authenticate.  It is not intended for use on the Internet.  It is intended for use on a local network, where the only people who can see the traffic are people you trust.

Do not, under any circumstances, use Frogpond on the Internet or any other unsecured network.

## Quick Start

- Build binaries:

    go build ./apps/server
    go build ./apps/client

- Run a server on one machine:

    ./server --name "Server 1"

- Start another server and point at the first as a peer:

    ./server --peer 192.0.2.10 --name "Server 2"

- Use the client to add/get values:

    ./client add testkey testvalue
    ./client get testkey

To install with `go install`, the module path is `gitlab.com/donomii/menu/frogpond`:

    go install gitlab.com/donomii/menu/frogpond/apps/server@latest
    go install gitlab.com/donomii/menu/frogpond/apps/client@latest

## Use

Frogpond can be used as a library, or as a stand-alone server and client.

For details, see commands below.

If you don't see your value on the second machine, try running

    ./client ips

to list the known peers.


The complete list of commands for the client:

    ips               - Print the ips of known frogpond servers
    dump              - Dump all data from the frogpond data pool
    add [key] [value] - Add a key/svalue pair to the frogpond data pool
    delete [key]      - Delete a key/value from the frogpond data pool
    get [key]         - Get a value from the frogpond data pool

## Library Use

Frogpond is available as a library, and is used as the core of the [clusterf](https://github.com/donomii/clusterf) project. Mainly you have to run `StartServer()`; it internally announces to peers and periodically syncs peer and key/value data. You can also call `UpdatePeers()` and `UpdatePeersData()` directly if embedding the library.

Data model notes:
- `DataPoint.Key` and `DataPoint.Value` are `[]byte`. When serialized to JSON they are base64‑encoded (standard Go `encoding/json` behavior). Use the helper methods on `Node` (e.g., `SetDataPointWithPrefix_str`) for common string workflows.

## Security

Frogpond communicates over HTTP, and does not authenticate or encrypt.  It is not secure.  It is intended for use on a local network, where the only people who can see the traffic are people you trust.

Do not use Frogpond on the Internet.

## Design

Frogpond communicates with other Frogpond servers using HTTP.  It uses a simple REST API to add, delete, and get key/value pairs.  It also uses a simple REST API to get the IP addresses of other Frogpond servers.

The APIs for P2P and data transfer are separate.  This allows Frogpond to be used as a simple host discovery service, and/or as a simple key/value store.

All values are kept in memory, there is no persistence.

Every time you modify the data pool, Frogpond updates its peers.  This means that if you add a key/value pair, it will be available to all peers within a few seconds.

Each Frogpond server regularly checks in with its peers, by default every 50s.  This allows updates to move through NATs and firewalls, but with a larger delay than usual.


## HTTP API

All endpoints are HTTP, unencrypted, unauthenticated, intended for trusted LANs only.

- GET `/public_info`
  - Response: JSON of this host’s public info:
    {"GUID":"<id>","Name":"<name>","Services":[{"Name":"...","Ip":"...","Port":16002,"Protocol":"tcp","Description":"...","Global":false,"Path":"/public_info"}]}

- POST `/contact`
  - Request body: JSON array of known hosts (can be empty `[]`).
  - Response: JSON array of this node’s known hosts.

- POST `/announce`
  - Request body: JSON object with this host’s `HostService` (ip will be set from connection).
  - Response: `OK` (text/plain).

- POST `/ponds/default`
  - Request body: JSON array of `DataPoint` updates.
  - Response: JSON array of this node’s datapool (may be `[]` if pull disabled).

Errors: Malformed JSON requests return HTTP 400 with a short error message.

## Building and Running

- Build: `go build ./...`
- Tests: `go test ./...`
- Race tests: `go test ./... -race`

Server flags:
- `--port`: listen port (default 16002)
- `--peer`: optional known peer ip/host
- `--name`: human‑friendly name

Graceful shutdown: the server handles SIGINT/SIGTERM and shuts down cleanly.

