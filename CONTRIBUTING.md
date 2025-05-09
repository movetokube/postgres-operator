# CONTRIBUTING

You can contribute to this project by opening a PR to merge to `master`, or one of the `vX.X.X` branches.

## Branching

`master` branch contains the latest source code with all the features. `vX.X.X` contains code for the specific major versions.
 i.e. `v0.4.x` contains the latest code for 0.4 version of the operator. See compatibility matrix below.

## Tests

Please write tests and fix any broken tests before you open a PR. Tests should cover at least 80% of your code.

## e2e-tests

End-to-end tests are implemented using [kuttl](https://kuttl.dev/), a Kubernetes test framework. To execute these tests locally, first install kuttl on your system, then run the command `make e2e` from the project root directory.
