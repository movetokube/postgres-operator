.PHONY: gen build e2e e2e-build

gen:
	operator-sdk generate k8s
	operator-sdk generate crds
build:
	operator-sdk build movetokube/postgres-operator
	docker push movetokube/postgres-operator
unit-test:
	go test ./... -coverprofile coverage.out
	go tool cover -func coverage.out
unit-test-coverage: unit-test
	go tool cover -html coverage.out
linux-docker:
	@docker run -ti -v $(PWD):/work golang:1.24-bookworm /bin/bash
linux-build:
	@GOBIN=/work/bin GO111MODULE=on GOOS=linux GOARC=x86_64 go build -o operator github.com/movetokube/postgres-operator/cmd/manager
docker-build:
	docker run -ti -v $(PWD):/work -w /work golang:1.24-bookworm make linux-build
e2e-build:
	docker buildx build -t postgres-operator:build -f ./build/Dockerfile.dist .
e2e: e2e-build
	kubectl kuttl test --config ./tests/kuttl-test-self-hosted-postgres.yaml
