# cf-k8s-api
This repository contains what we call the "CF API Shim", an experimental implementation of the V3 Cloud Foundry API that is backed entirely by Kubernetes [custom resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/).

For more information about what we're building, check out the [Vision for CF on Kubernetes](https://docs.google.com/document/d/1rG814raI5UfGUsF_Ycrr8hKQMo1RH9TRMxuvkgHSdLg/edit) document.

## Documentation
While this project seeks to faithfully reproduce the [V3 CF APIs](https://v3-apidocs.cloudfoundry.org/version/release-candidate/) as much as is feasible, not all APIs have been implemented and not all features are supported.

We maintain our own set of [API Endpoint docs](docs/api.md) for tracking what is currently supported by the Shim.

## Installation

### Dependencies
This project relies on the controllers and CRDs of the [cf-k8s-controllers](https://github.com/cloudfoundry/cf-k8s-controllers) repo. Install it by following the instructions in its [README](https://github.com/cloudfoundry/cf-k8s-controllers/blob/main/README.md).

### Running Locally
make
```make
make run
```
shell
```shell
go run main.go
```

### Deploying to a Cluster
Support for deploying the Shim to a Kubernetes cluster is coming soon.

## Contributing

### Running Tests
make
```make
make test
```
shell (testbin must be sourced first if using this method)
```shell
KUBEBUILDER_ASSETS=$PWD/testbin/bin go test ./... -coverprofile cover.out
```

### Updating CRDs for Tests
Some tests run a real Kubernetes API/etcd via the [`envtest`](https://book.kubebuilder.io/reference/envtest.html) package. These tests rely on the CRDs from [cf-k8s-controllers](https://github.com/cloudfoundry/cf-k8s-controllers) which we have vendored in.
To update these CRDs you'll need to install [vendir](https://carvel.dev/vendir/) and run `vendir sync` in the `repositories/fixtures` directory.

### Editing Local Configuration
To specify a custom configuration file, set the `CONFIG` environment variable to its path when running the web server.
Refer to the [default config](config.json) for the config file structure and options.

## Regenerate kubernetes resources after making changes
To regenerate the kubernetes resources under `./config`, run `make generate` or `go generate ./...`
from the root of the project.

## Deploying the app to your cluster
You can deploy the app to your cluster by running `kubectl apply -f ./config -R`
from the project root.
