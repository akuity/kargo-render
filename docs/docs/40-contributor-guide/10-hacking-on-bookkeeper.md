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
