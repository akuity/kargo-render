---
title: Go module
description: Using Bookkeeper's Go module
---

# Using Bookkeeper's Go module

Bookkeeper's functionality is available as a [Go](https://go.dev/) module.

To add the module to your project:

```shell
go get github.com/akuityio/bookkeeper
```

Then instantiate an implementation of the `bookkeeper.Service` interface and
invoke the `RenderConfig()` function:

```golang
import "github.com/akuityio/bookkeeper"

// ...

svc := bookkeeper.NewService(
  &bookkeeper.ServiceOptions{
    LogLevel: bookkeeper.LogLevelDebug,
  },
)

res, err := svc.RenderConfig(
  context.Background(),
  bookkeeper.RenderRequest{
    RepoURL: "https://<repo URL>",
    RepoCreds: bookkeeper.RepoCredentials{
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

Unless an error occurs, the response (`bookkeeper.RenderResponse`) from the call
above will contain details of any commit or pull request created by Bookkeeper.

:::tip
If options are omitted from the call to `bookkeeper.NewService()` (e.g. `nil`
is passed), the default log level is `bookkeeper.LogLevelError`.
:::

:::tip
Compatible binaries for Git, Kustomize, ytt, and Helm must be available when
using this module. Consider using Bookkeeper's official Docker image as a base
image for your own software. This will ensure the availability of compatible
binaries.
:::
