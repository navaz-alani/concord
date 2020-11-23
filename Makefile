.PHONY: echo-all
# Targets for examples/echo
echo-all: echo-server echo-client echo-load-client
echo-server: $(wildcard ./examples/echo/server/*.go)
	go build -o $@ ./examples/echo/server
echo-client: $(wildcard ./examples/echo/client/*.go)
	go build -o $@ ./examples/echo/client
echo-load-client: $(wildcard ./examples/echo/load-client/*.go)
	go build -o $@ ./examples/echo/load-client/
