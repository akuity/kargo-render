---
title: Docker image
description: Using Bookkeeper's Docker image
---

# Using Bookkeeper's Docker image

If you are integrating Bookkeeper into automated processes that are implemented
with something other than GitHub Actions and those processes permit execution of
commands within a Docker container (much as a GitHub action does), then
utilizing the official Bookkeeper Docker image and the CLI found therein is a
convenient option.

As long as Docker is installed on your system, using this image is also the
easiest option for experimenting locally with Bookkeeper!

Example usage:

```shell
docker run -it ghcr.io/akuityio/bookkeeper-prototype:v0.1.0-alpha.2-rc.8 \
  bookkeeper render \
  --repo https://github.com/<your GitHub handle>/bookkeeper-demo-deploy \
  --repo-username <your GitHub handle> \
  --repo-password <a GitHub personal access token> \
  --target-branch env/dev
```

:::tip
Although the exact procedure for emulating the example above will vary from one
automation platform to the next, the Bookkeeper image should permit you to
integrate Bookkeeper with a broad range of automation platforms including, but
not limited to, popular choices such as [CircleCI](https://circleci.com/) or
[Travis CI](https://www.travis-ci.com/).
:::

:::caution
The `bookkeeper` CLI is not designed to be run anywhere except within a
container based on the official Bookkeeper image. The official Bookkeeper image
provides compatible versions of Kustomize, ytt, and Helm that cannot be
guaranteed to exist on other systems.
:::

:::tip
If you're using Bookkeeper's [Go module](./go-module) to interact
programmatically with Bookkeeper, you might _also_ consider utilizing the
Bookkeeper Docker image as a base image for your own software since it will
guarantee the existence of compatible versions of Kustomize, ytt, and Helm. 
:::