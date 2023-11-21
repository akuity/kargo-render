---
slug: /
title: Overview
description: What is Kargo Render?
---

# What is Kargo Render?

Kargo Render is a sub-project of [Kargo](https://github.com/akuity/kargo)
that helps Kubernetes users incorporate the environment branches pattern
into their [GitOps](https://opengitops.dev/) practice. There are numerous
benefits to this pattern, which you can read about in detail
[here](./environment-branches). If you're already familiar with this pattern and
convinced of its benefits, dive right into the next section to get started!

:::caution
Kargo Render is highly experimental at this time and breaking changes should be
anticipated between pre-GA minor releases.
:::

## What you need to know

Kargo Render does _one thing:_ It uses your preferred configuration management
tool and some simple rules you define to render configuration from the default
branch (e.g. `main`) of a remote GitOps repository into plain manifests that it
stores in environment-specific branches of the same repository.

When invoking Kargo Render, you only need to specify the URL of the GitOps
repository, appropriate credentials, and the name of the environment branch for
which you wish to render and store manifests. Kargo Render does the rest and you
can point applicable configuration of your preferred GitOps-enabled CD platform
at the environment branch.

## Kargo Render & Argo CD

Kargo Render is compatible with Argo CD manifest rendering. Behind the scenes, Kargo Render uses
Argo CD repo server to generate final manifests. That includes the same config management settings as well as features
like automatic tool detection and
[git parameter overrides](https://argo-cd.readthedocs.io/en/stable/user-guide/parameters/#store-overrides-in-git).
This ensures painless back-and-forth switching between native Argo CD manifest generation and Kargo Render. 

## Getting started

[Kargo](https://kargo.akuity.io/), the application lifecycle platform
for Kubernetes, leverages Kargo Render to orchestrate manifests changes promotion.
You can also use Kargo Render as a standalone tool:


1. Kargo Render can be integrated into your GitOps practice in a variety of
   ways. Regardless of your entrypoint into its functionality, it relies on a
   common bit of configuration -- `kargo-render.yaml`. Read more about that
   [here](./how-to-guides/configuration).

1. Once you've configured Kargo Render, you have several options for how to
   utilize it. Skip right to the how-to section that best addresses your use
   case.

    * [GitHub Actions](./how-to-guides/github-actions): Start here if you want
      to incorporate Kargo Render into workflows powered by GitHub Actions.

    * [Docker image](./how-to-guides/docker-image): Start here if you want to
      incorporate Kargo Render into workflows in _any other_ container-enabled
      automation platform. This is also the easiest option for experimenting
      with Kargo Render locally!

    * [Go module](./how-to-guides/go-module): Start here if you want to
      integrate with Kargo Render programmatically using [Go](https://go.dev/)
      code. This is especially relevant for developers looking to build more
      sophisticated GitOps tools on top of a foundation provided by Kargo
      Render.
