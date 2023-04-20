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

Example usage:

```yaml
jobs:
  render-test-manifests:
    name: Render test manifests
    runs-on: ubuntu-latest
    steps:
    - name: Render manifests
      uses: akuity/akuity-bookkeeper@v0.1.0-alpha.2-rc.14
      with:
        personalAccessToken: ${{ secrets.GITHUB_TOKEN }}
        targetBranch: env/test
```

In the example `render-test-manifests` job above, you can see that rendering
manifests into an environment-specific branch requires little more than
providing a token, and specifying the branch name. The action takes care of the
rest.

:::note
`secrets.GITHUB_TOKEN` is automatically available in every GitHub Actions
workflow and, depending on repository settings, may have sufficient permissions
to both read from and write to your repository. If this is not the case, you can
update repository settings. You can read more about this
[here](https://docs.github.com/en/actions/security-guides/automatic-token-authentication#permissions-for-the-github_token).
:::
