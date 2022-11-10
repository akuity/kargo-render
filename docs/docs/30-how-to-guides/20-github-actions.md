---
title: GitHub Actions
description: Using Bookkeeper with GitHub Actions
---

# Using Bookkeeper with GitHub Actions

If you are integrating Bookkeeper into workflows that are implemented via GitHub
Actions, Bookkeeper can be run as an action.

:::info
The Bookkeeper action utilizes the official Bookkeeper Docker image and
therefore has guaranteed access to compatible versions of
Git, Helm, Kustomize, and ytt, which are included on that image.
:::

## Installing the action

:::info
As of this writing, GitHub Actions does not have good support for _private_
actions. This being the case, some extra setup is currently required in order to
use the Bookkeeper action.
:::

Paste the following YAML, verbatim into `.github/actions/bookkeeper` in your
GitOps repository:

```yaml
name: 'Bookkeeper'
description: 'Publish rendered config to an environment-specific branch'
inputs:
  personalAccessToken:
    description: 'A personal access token that allows Bookkeeper to write to your repository'
    required: true
  targetBranch:
    description: 'The environment-specific branch for which you want to render configuration'
    required: true
runs:
  using: 'docker'
  image: 'krancour/mystery-image:v0.1.0-alpha.1'
  entrypoint: 'bookkeeper-action'
```

:::note
The odd-looking reference to a Docker image named
`krancour/mystery-image:v0.1.0-alpha.1` is not a mistake. As previously
noted, GitHub support for private actions is very poor. Among other things, this
means there is no method of authenticating to a Docker registry to pull private
images. `krancour/mystery-image:v0.1.0-alpha.1` is a public copy of the
official Bookkeeper image. We hope that its obscure name prevents it from
attracting much notice.
:::

## Using the action

Because the action definition exists within your own repository (see previous
section), you must utilize
[actions/checkout](https://github.com/marketplace/actions/checkout) to ensure
that definition is available during the execution of your workflow. After doing
so, the Bookkeeper action is as easy to use as if it had been sourced from the
GitHub Actions Marketplace.

:::info
In the future, this step will not be required.
:::

Example usage:

```yaml
jobs:
  render-dev-manifests:
    name: Render dev manifests
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v3
      - name: Render manifests
        uses: ./.github/actions/bookkeeper/
        with:
          personalAccessToken: ${{ secrets.GITHUB_TOKEN }}
          targetBranch: env/dev
```

In the example `render-dev-manifests` job above, you can see that rendering
configuration into an environment-specific branch requires little more than
providing a token, and specifying the branch name. The action takes care of the
rest.

:::note
`secrets.GITHUB_TOKEN` is automatically available in every GitHub Actions
workflow and should have sufficient permissions to both read from and write to
your repository.
:::
