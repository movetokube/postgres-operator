.PHONY: gen build

gen:
	operator-sdk generate k8s
	operator-sdk generate crds
build:
	operator-sdk build movetokube/postgres-operator
	docker push movetokube/postgres-operator