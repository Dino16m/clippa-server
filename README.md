# Clippa Server

Clippa server is the backend service for a clipboard sharing client. It facilitates the creation and management of "parties" which are secure groups for real-time clipboard sharing between members using WebSockets.

## Features

- **Party Management**: Create, get, and authenticate into parties.
- **Secure Communication**: Generates a unique CA bundle for each party to secure communication.
- **Real-time Communication**: Uses WebSockets for real-time messaging and clipboard updates between party members.
- **Leader Election**: Implements a conclave and voting mechanism for leader election within a party.

## Getting Started

### Prerequisites

- Go 1.25.5 or later
- A running database instance (SQLite is used by default)

### Installation

1.  **Clone the repository**:

    ```sh
    git clone https://github.com/dino16m/clippa-server.git
    cd clippa-server
    ```

2.  **Install dependencies**:
    ```sh
    go mod download
    ```

### Configuration

The server can be configured using environment variables. The following configuration options are available:

- **PORT**: The port on which the server will run. Defaults to `8080`.
- **DATABASE_URL**: The connection string for the database. Defaults to `clippa.db`.

Create a `config.yaml` file in the root of the project with the following content:

```yaml
PORT: 8080
DATABASE_URL: clippa.db
```

Alternatively, you can set these values as environment variables:

```sh
export PORT=8080
export DATABASE_URL=clippa.db
```

### Running the Server

To run the server, execute the following command from the root of the project:

```sh
go run ./cmd/main.go
```

## Docker

You can also run the server in a Docker container.

### Build the Image

To build the Docker image, run the following command from the root of the project:

```sh
docker build -t clippa-server .
```

### Run the Container

To run the Docker container, use the following command:

```sh
docker run -p 8080:8080 -d clippa-server
```

This will start the server and map port 8080 on your host to port 8080 in the container.

## API Reference

### Create Party

- **Endpoint**: `POST /parties/`
- **Description**: Creates a new party.
- **Request Body**:
  ```json
  {
    "name": "my-party",
    "secret": "my-secret"
  }
  ```
- **Response**:
  ```json
  {
    "id": "...",
    "name": "my-party",
    "certPem": "...",
    "keyPem": "..."
  }
  ```

### Get Party

- **Endpoint**: `GET /parties/`
- **Description**: Retrieves party information.
- **Query Parameters**:
  - `id`: The ID of the party.
- **Headers**:
  - `X-Secret`: The secret of the party.
- **Response**:
  ```json
  {
    "id": "...",
    "name": "my-party",
    "certPem": "...",
    "keyPem": "..."
  }
  ```

### Authenticate

- **Endpoint**: `GET /parties/auth/`
- **Description**: Authenticates a user and returns a token for joining a party.
- **Query Parameters**:
  - `id`: The ID of the party.
- **Headers**:
  - `X-Secret`: The secret of the party.
- **Response**:
  ```json
  {
    "token": "..."
  }
  ```

### Join Party

- **Endpoint**: `GET /parties/join/`
- **Description**: Joins a party using a WebSocket connection.
- **Query Parameters**:
  - `id`: The ID of the party.
  - `token`: The authentication token.
  - `memberId`: (Optional) A unique ID for the member.

## WebSocket Communication

Once a WebSocket connection is established, clients can send and receive messages of the following types:

- `conclave`: A message containing a list of member addresses for leader election.
- `ping`: A message to check the liveness of a connection.
- `pong`: A response to a ping message.
- `vote`: A message containing ballots for leader election.
- `set-leader`: A message to set the leader of the party. Setting a leader allows clients to designate a local address reachable to all the clients and allows clients to take the party to their local network.
- `leader-elected`: A message to notify party members of the poll result
- `clipboard`: A message containing clipboard content.
- `joined`: A notification that a member has joined the party.
- `left`: A notification that a member has left the party.
- `error`: A message containing an error.
