.PHONY: dev build start stop restart migrate

OUTFILE=pionus
PIDFILE=pionus.pid

dev:
	go run .

build:
	go build -o $(OUTFILE) .

stop:
ifneq (,$(wildcard $(PIDFILE)))
	kill -9 $$(cat $(PIDFILE))
	rm -f $(PIDFILE)
endif

start: build
	./$(OUTFILE) & echo $$! > $(PIDFILE)

restart: stop start

migrate:
	go run . -migrate
