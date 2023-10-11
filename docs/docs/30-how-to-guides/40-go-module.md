---
title: Go module
description: Using Kargo Render's Go module
---

# Using Kargo Render's Go module

Kargo Render's functionality is available as a [Go](https://go.dev/) module.

To add the module to your project:

```shell
go get github.com/akuity/kargo-render
```

Then instantiate an implementation of the `render.Service` interface and
invoke the `RenderManifests()` function:

```golang
import "github.com/akuity/kargo-render"

// ...

svc := render.NewService(
  &render.ServiceOptions{
    LogLevel: render.LogLevelDebug,
  },
)

res, err := svc.RenderManifests(
  context.Background(),
  render.RenderRequest{
    RepoURL: "https://<repo URL>",
    RepoCreds: render.RepoCredentials{
      Username: "<username>",
      Password: "<password or personal access token>",
    },
    Commit:       "<sha>",                         // Optional
    TargetBranch: "env/dev",                       // For example
    Images:       []string{"my-new-image:v0.1.0"}, // Optional
  },
)
if err != nil {
  // Handle err
}
```

Unless an error occurs, the response (`render.RenderResponse`) from the call
above will contain details of any commit or pull request created by Kargo
Render.

:::tip
If options are omitted from the call to `render.NewService()` (e.g. `nil`
is passed), the default log level is `render.LogLevelError`.
:::

:::tip
Compatible binaries for Git, Kustomize, ytt, and Helm must be available when
using this module. Consider using Kargo Render's official Docker image as a base
image for your own software. This will ensure the availability of compatible
binaries.
:::
