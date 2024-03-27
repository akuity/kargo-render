---
title: Docker image
description: Using Kargo Render's Docker image
---

# Using Kargo Render's Docker image

If you are integrating Kargo Render into automated processes that are
implemented with something other than GitHub Actions and those processes permit
execution of commands within a Docker container (much as a GitHub action does),
then utilizing the official Kargo Render Docker image and the CLI found therein
is a convenient option.

As long as Docker is installed on your system, using this image is also the
easiest option for experimenting locally with Kargo Render!

Example usage:

```shell
docker run -it ghcr.io/akuity/kargo-render:v0.1.0-rc.39 \
  --repo https://github.com/<your GitHub handle>/kargo-render-demo-deploy \
  --repo-username <your GitHub handle> \
  --repo-password <a GitHub personal access token> \
  --target-branch env/dev
```

:::tip
Although the exact procedure for emulating the example above will vary from one
automation platform to the next, the Kargo Render image should permit you to
integrate Kargo Render with a broad range of automation platforms including, but
not limited to, popular choices such as [CircleCI](https://circleci.com/) or
[Travis CI](https://www.travis-ci.com/).
:::

:::caution
The `kargo-render` CLI is not designed to be run anywhere except within a
container based on the official Kargo Render image. The official Kargo Render
image provides compatible versions of Kustomize and Helm that cannot be
guaranteed to exist on other systems.
:::

:::tip
If you're using Kargo Render's [Go module](./go-module) to interact
programmatically with Kargo Render, you might _also_ consider utilizing the
Kargo Render Docker image as a base image for your own software since it will
guarantee the existence of compatible versions of Kustomize, and Helm. 
:::
