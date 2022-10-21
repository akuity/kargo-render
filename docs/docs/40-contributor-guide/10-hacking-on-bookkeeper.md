---
description: Hacking on Bookkeeper
---

# Hacking on Bookkeeper

Bookkeeper is implemented in Go. For maximum productivity in your text editor or
IDE, it is recommended that you have installed the latest stable releases of Go
and applicable editor/IDE extensions, however, this is not strictly required to
be successful.

## Containerized tests

In order to minimize the setup required to successfully apply small changes and
in order to reduce the incidence of “it worked on my machine,” wherein changes
that pass tests locally do not pass the same tests in CI due to environmental
differences, Bookkeeper has adopted a “container-first” approach to testing.
This is to say we have made it the default that unit tests, linters, and a
variety of other validations, when executed locally, automatically execute in a
Docker container that is maximally similar to the container in which those same
tasks will run during the continuous integration process.

To take advantage of this, you only need to have
[Docker](https://docs.docker.com/engine/install/) and `make` installed.

If you wish to opt-out of tasks automatically running inside a container, you
can set the environment variable `SKIP_DOCKER` to the value `true`. Doing so
will require that any tools involved in tasks you execute have been installed
locally. 

## Testing Bookkeeper code

If you make modifications to the code base, it is recommended that you run
unit tests and linters before opening a PR.

To run all unit tests:

```shell
make test-unit
```

To run lint checks:

```shell
make lint
```

## Building and running the server from source

The Bookkeeper server runs as a Docker container. To build the image from
source for your system's native architecture only, run the following:

```shell
make hack-build-image
```

This is hardcoded to produce an image tagged as `bookkeeper:dev`.

To build the image and run the server in just a single step:

```shell
make hack-run-server
```

By default, this will expose the Bookkeeper server at `http://localhost:8080`.
If you would like to bind to a different port, do so by setting the
`BOOKKEEPER_SERVER_PORT` environment variable like so:

```shell
BOOKKEEPER_SERVER_PORT=<port> make hack-run-server
```

:::note
These targets exist purely to facilitate locally trying changes you have applied
to the Bookkeeper server. They are optimized for simplicity and speed. The
actual build process produces images (from the same `Dockerfile`) for multiple
CPU architectures.
:::

## Building and running the CLI from source

To build the `bookkeeper` CLI from source for your system's native OS and
architecture only, run the following:

```shell
make hack-build-cli
```

This will produce a binary in `bin/bookkeeper-<os>-<architecture>`.

:::note
This target exist purely to facilitate locally trying changes you have applied
to Bookkeeper. It is optimized for simplicity and speed. The actual build
process produces binaries for multiple operating systems and CPU architectures.
:::