# Targets for examples/echo
echo-server: $(wildcard ./examples/echo/server/*.go)
	go build -o $@ ./examples/echo/server
echo-client: $(wildcard ./examples/echo/client/*.go)
	go build -o $@ ./examples/echo/client
