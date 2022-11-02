---
title: Rendered YAML branches
description: What are rendered YAML branches?
---

# What are rendered YAML branches?

Understanding the _rendered YAML branches_ pattern at the heart of Bookkeeper
begins with understanding some common difficulties encountered by
[GitOps](https://opengitops.dev/) practitioners.

## Configuration management

To keep Kubernetes manifests concise and manageable, most GitOps practitioners
incorporate some manner of configuration management tooling into their
deployments. [Kustomize](https://kustomize.io/), [ytt](https://carvel.dev/ytt/),
and [Helm](https://helm.sh/) are three popular examples of such tools. Although
they may employ widely varied approaches, tools in this class all enable the
same fundamental capability -- maintaining a common set of "base" configuration
that can be amended or patched in some way to suit each of the (potentially
many) environments to which you might deploy your application.

Continuous delivery platforms, like [Argo CD](https://argoproj.github.io/cd/) or
[Flux](https://fluxcd.io/), commonly have built-in awareness of tools such as
these. Argo CD, for instance, can easily detect the use of Kustomize or Helm
(but not ytt) and utilize embedded versions of those tools to _render_ such
configuration into _plain YAML_ that is appropriate for a given environment.
While, at a glance, this may seem convenient, relying on these integrations to
perform just-in-time rendering of your configuration also poses some significant
drawbacks. Notably:

* The source of truth for your application's configuration (the main branch of
  your GitOps repository) can be _obfuscated_ by your tooling. Since you don't
  see the plain YAML that will be applied to your cluster(s) _before_ it's
  applied, an accurate notion of what you are deploying is dependent upon your
  ability to mentally render plain YAML precisely as your tools will.

* Upgrades to your CD platform may include upgrades to embedded configuration
  management tools. Changes in those tools may alter the _interpretation_ of
  what you consider your source of truth. i.e. Plain YAML rendered from the
  contents of your main branch tomorrow could _differ_ from what was rendered
  from the same input today. _If your source of truth is subject to
  interpretation, that truth is not objective._

## Rendered YAML branches

The _rendered YAML branches_ pattern can alleviate the problems highlighted in
the previous section. Implementing this pattern simply means the main branch of
your application's GitOps repository ceases to be the source of truth and
becomes, instead, an _input_ to tools that will _render the truth as plain YAML
and persist it to dedicated, environment-specific branches of your GitOps
repository._

For any application, this pattern:

* Creates a comprehensive, one-to-one mapping between branches of your GitOps
  repository and corresponding environments.

* Deobfuscates what's deployed to each environment.

* _Puts you in control by making the most of GitOps._ Apply features of your
  Git provider, such as GitHub
  [branch protection rules](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/defining-the-mergeability-of-pull-requests/managing-a-branch-protection-rule)
  and [GitHub Actions](https://github.com/features/actions), to implement
  suitable policies and workflows _on a per-environment basis_.

Despite its many advantages, the rendered YAML branches pattern can be onerous
to implement because it requires additional automation to effectively "promote"
changes made to your main branch into a given environment by rendering
configuration into the corresponding branch. Bookkeeper's singular goal is to
answer those difficulties with an intuitive and opinionated (yet sensible) tool
that puts the benefits of the rendered YAML branches pattern easily within reach
for all GitOps practitioners.
