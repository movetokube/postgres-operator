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
