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