.PHONY: gen build

gen:
	operator-sdk generate k8s
	operator-sdk generate crds
build:
	operator-sdk build movetokube/postgres-operator
	docker push movetokube/postgres-operator
unit-test:
	go test ./... -mod vendor -coverprofile coverage.out
	go tool cover -func coverage.out
unit-test-coverage: unit-test
	go tool cover -html coverage.out
linux-docker:
	@docker run -ti -v $(PWD):/work golang:1.22-alpine /bin/bash
linux-build:
	@GOBIN=/work/bin GO111MODULE=on GOOS=linux GOARC=x86_64 go build --mod=vendor  -o operator github.com/movetokube/postgres-operator/cmd/manager
docker-build:
	docker run -ti -v $(PWD):/work -w /work golang:1.22-alpine make linux-build
