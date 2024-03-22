---
title: GitHub Actions
description: Using Kargo Render with GitHub Actions
---

# Using Kargo Render with GitHub Actions

If you are integrating Kargo Render into workflows that are implemented via
GitHub Actions, Kargo Render can be run as an action.

:::info
The Kargo Render action utilizes the official Kargo Render Docker image and
therefore has guaranteed access to compatible versions of
Git, Helm, and Kustomize, which are included on that image.
:::

Example usage:

```yaml
permissions: 
  contents: write
  pull-requests: write

jobs:
  render-test-manifests:
    name: Render test manifests
    runs-on: ubuntu-latest
    steps:
    - name: Render manifests
      uses: akuity/kargo-render-action@v0.1.0-rc.34
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
workflow and, depending on your workflow and repository settings, may have
sufficient permissions to both read from and write to your repository. If
this is not the case, you can update repository settings. You can read more
about this [here](https://docs.github.com/en/actions/security-guides/automatic-token-authentication#permissions-for-the-github_token).
:::
