.PHONY: dev build start stop restart migrate test vet fmt tidy docker audit audit-online

OUTFILE=pionus
PIDFILE=pionus.pid

dev:
	go run .

build:
	go build -o $(OUTFILE) .

stop:
ifneq (,$(wildcard $(PIDFILE)))
	kill -9 $$(cat $(PIDFILE)) 2>/dev/null || true
	rm -f $(PIDFILE)
endif

start: build
	./$(OUTFILE) & echo $$! > $(PIDFILE)

restart: stop start

migrate:
	go run . -migrate

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './theme/*')

tidy:
	go mod tidy

docker:
	docker build -t pionus .

audit:
	go run . -audit-images

audit-online:
	go run . -audit-images -audit-images-online
