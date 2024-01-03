---
description: Hacking on Kargo Render
---

# Hacking on Kargo Render

Kargo Render is implemented in Go. For maximum productivity in your text editor
or IDE, it is recommended that you have installed the latest stable releases of
Go and applicable editor/IDE extensions, however, this is not strictly required
to be successful.

## Running tests

In order to minimize the setup required to successfully apply small changes and
in order to reduce the incidence of “it worked on my machine,” wherein changes
that pass tests locally do not pass the same tests in CI due to environmental
differences, Kargo Render has made it trivial to execute tests within a
container that is maximally similar to the containers that tests execute in
during the continuous integration process.

To take advantage of this, you only need to have
[Docker](https://docs.docker.com/engine/install/) and `make` installed.

To run all unit tests:

```shell
make hack-test-unit
```

:::info
If you wish to opt-out of executing the tests within a container, use the
following instead:

```shell
make test-unit
```

This will require Go to be installed locally.
:::

To run lint checks:

```shell
make hack-lint
```

:::info
If you wish to opt-out of executing the linter within a container, use the
following instead:

```shell
make lint
```

This will require Go and [golangci-lint](https://golangci-lint.run/) to be
installed locally.
:::

## Building the image

To build source into a Docker image that will be tagged as `kargo-render:dev`,
execute the following:

```shell
make hack-build
```

:::note
Because Kargo Render is dependent on compatible versions of Git, Kustomize,
and Helm binaries, there is seldom, if ever, a reason to build or execute the
Kargo Render binaries outside the context of a container that provides those
dependencies.
:::
