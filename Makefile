.PHONY: gen build

gen:
	operator-sdk generate k8s
build:
	operator-sdk build hitman99/postgres-operator
	docker push hitman99/postgres-operator