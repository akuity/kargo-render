---
slug: /
title: Overview
description: What is Bookkeeper?
---

# What is Bookkeeper?

Bookkeeper helps Kubernetes users incorporate the rendered branches pattern
into their CD workflows and [GitOps](https://opengitops.dev/) practice. There
are numerous benefits to this pattern, which you can read about in detail
[here](./rendered-branches). If you're already familiar with this pattern and
convinced of its benefits, dive right into the next section to get started!

:::caution
Bookkeeper is highly experimental at this time and breaking changes should be
anticipated between pre-GA minor releases.
:::

## What you need to know

Bookkeeper does _one thing:_ It uses your preferred configuration management
tool to render configuration from the default branch of a remote GitOps
repository into _plain YAML_ that it stores in an environment-specific "target
branch" of the same repository. You only specify the URL of the repository,
appropriate credentials, and the name of the target branch. Bookkeeper does the
rest and you can point your preferred GitOps-enabled CD platform at the target
branch.

When rendering configuration, Bookkeeper exposes only two options, which may or
may not be useful to you, depending on your workflows:

* You may specify the ID of a commit from which to render configuration. When
  unspecified, Bookkeeper defaults to rendering configuration from the head of
  your GitOps repository's default branch.

* You may specify the name of a new Docker image or a list of image names to
  replace older images referenced by your configuration.

## Getting started

1. Bookkeeper can be integrated into your CD workflows in a variety of ways.
   Regardless of your entrypoint into its functionality, it relies on you
   employing a compatible layout in your GitOps repository. Read about that
   layout [here](./how-to-guides/repository-layout).

1. Once you're using a Bookkeeper-compatible repository layout, you have several
   options for how to utilize Bookkeeper. Skip right to the how-to section that
   best addresses your use case.

    * [GitHub Actions](./how-to-guides/github-actions): Start here if you want
      to incorporate Bookkeeper into workflows powered by GitHub Actions.

    * [Docker image](./how-to-guides/docker-image): Start here if you want to
      incorporate Bookkeeper into workflows in _any other_ container-enabled
      automation platform. This is also the easiest option for experimenting
      locally with Bookkeeper!

    * [Go module](./how-to-guides/go-module): Start here if you want to
      integrate with Bookkeeper programmatically using [Go](https://go.dev/)
      code. This is especially relevant for developers looking to build more
      sophisticated GitOps tools on top of a foundation provided by Bookkeeper.
