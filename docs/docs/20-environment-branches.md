---
title: Environment branches
description: What are environment branches?
---

# What are environment branches?

Understanding the _environment branches_ pattern at the heart of Kargo Render
begins with understanding some common difficulties encountered by
[GitOps](https://opengitops.dev/) practitioners.

## Configuration management

To keep Kubernetes manifests concise and manageable, most GitOps practitioners
incorporate some manner of configuration management tooling into their
deployments. [Kustomize](https://kustomize.io/),
and [Helm](https://helm.sh/) are two popular examples of such tools. Although
they may employ widely varied approaches, tools in this class all enable the
same fundamental capability -- maintaining a common set of "base" configuration
that can be amended or patched in some way to suit each of the environments to
which you might deploy your application.

Continuous delivery platforms, like [Argo CD](https://argoproj.github.io/cd/) or
[Flux](https://fluxcd.io/), commonly integrate with tools such as these. Argo
CD, for instance, can easily detect the use of Kustomize or Helm
and utilize embedded versions of those tools to render such configuration into
plain manifests that are appropriate for a given environment. While, at a
glance, this may seem convenient, relying on these integrations to perform
just-in-time rendering of your manifests also poses some significant drawbacks.
Notably:

* The source of truth for your application's manifests (e.g. the `main` branch
  of your GitOps repository) can be _obfuscated_ by your tooling. Since you
  don't see the plain manifests that will be applied to a given environment
  _before_ they're applied, any notion of what you are actually deploying to
  that environment is dependent upon your ability to mentally render those
  manifests precisely as your tools will.

* Upgrades to your CD platform may include upgrades to embedded configuration
  management tools. Changes in those tools may alter the _interpretation_ of
  what you consider your source of truth. i.e. Plain manifests rendered from the
  contents of your `main` branch tomorrow could _differ_ from what was rendered
  from the same input today. _If your source of truth is subject to
  interpretation, that truth is not objective._

## Environment branches

The _environment branches_ pattern can alleviate the problems highlighted in the
previous section. Implementing this pattern simply means the `main` branch of
your application's GitOps repository _ceases to be the source of truth_ and
becomes, instead, an _input_ to tools that will _render the truth as plain
manifests and persist them to environment-specific branches._

For any application, this pattern:

* Creates a comprehensive, one-to-one mapping between branches of your GitOps
  repository and corresponding environments.

* Deobfuscates what's deployed to each environment.

* _Puts you in control by making the most of GitOps._ Apply features of your
  Git provider, such as pull requests, GitHub
  [branch protection rules](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/defining-the-mergeability-of-pull-requests/managing-a-branch-protection-rule)
  and [GitHub Actions](https://github.com/features/actions), to implement
  suitable policies and workflows _on a per-environment basis_.

Despite its many advantages, the environment branches pattern can be onerous
to implement because it requires new automation to continuously render
changes to your `main` branch into your environment branches. Kargo Render's
singular goal is to answer those difficulties with an intuitive tool that puts
the benefits of the environment branches pattern easily within reach for all
GitOps practitioners.
