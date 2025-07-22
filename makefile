protogen-wrkr:
	scripts/genproto.sh api/worker.proto

protogen-orc:
	scripts/genproto.sh api/orchestrator.proto

test:
	go test ./...

testv:
	go test -v ./...

testc:
	go test -cover ./...


build:
	docker-compose build

run:
	docker-compose up -d