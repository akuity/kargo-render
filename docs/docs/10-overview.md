---
slug: /
title: Overview
description: What is Bookkeeper?
---

# What is Bookkeeper?

Bookkeeper helps Kubernetes users incorporate the environment branches pattern
into their [GitOps](https://opengitops.dev/) practice. There are numerous
benefits to this pattern, which you can read about in detail
[here](./environment-branches). If you're already familiar with this pattern and
convinced of its benefits, dive right into the next section to get started!

:::caution
Bookkeeper is highly experimental at this time and breaking changes should be
anticipated between pre-GA minor releases.
:::

## What you need to know

Bookkeeper does _one thing:_ It uses your preferred configuration management
tool and some simple rules you define to render configuration from the default
branch (e.g. `main`) of a remote GitOps repository into plain manifests that it
stores in environment-specific branches of the same repository.

When invoking Bookkeeper, you only need to specify the URL of the GitOps
repository, appropriate credentials, and the name of the environment branch for
which you wish to render and store manifests. Bookkeeper does the rest and you
can point applicable configuration of your preferred GitOps-enabled CD platform
at the environment branch.

## Getting started

1. Bookkeeper can be integrated into your GitOps practice in a variety of ways.
   Regardless of your entrypoint into its functionality, it relies on a
   common bit of configuration -- `Bookfile.yaml`. Read more about that
   [here](./how-to-guides/configuration).

1. Once you've configured Bookkeeper, you have several options for how to
   utilize it. Skip right to the how-to section that best addresses your use
   case.

    * [GitHub Actions](./how-to-guides/github-actions): Start here if you want
      to incorporate Bookkeeper into workflows powered by GitHub Actions.

    * [Docker image](./how-to-guides/docker-image): Start here if you want to
      incorporate Bookkeeper into workflows in _any other_ container-enabled
      automation platform. This is also the easiest option for experimenting
      with Bookkeeper locally!

    * [Go module](./how-to-guides/go-module): Start here if you want to
      integrate with Bookkeeper programmatically using [Go](https://go.dev/)
      code. This is especially relevant for developers looking to build more
      sophisticated GitOps tools on top of a foundation provided by Bookkeeper.
