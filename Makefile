.PHONY: echo-all crypto-all clean
# Targets for examples/echo
echo-all: echo-server echo-client echo-load-client
echo-server: $(wildcard ./examples/echo/server/*.go)
	go build -o $@ ./examples/echo/server
echo-client: $(wildcard ./examples/echo/client/*.go)
	go build -o $@ ./examples/echo/client
echo-load-client: $(wildcard ./examples/echo/load-client/*.go)
	go build -o $@ ./examples/echo/load-client/
# Targets for examples/crypto
crypto-all: crypto-server crypto-client
crypto-server: $(wildcard ./examples/crypto/server/*.go)
	go build -o $@ ./examples/crypto/server
crypto-client: $(wildcard ./examples/crypto/client/*.go)
	go build -o $@ ./examples/crypto/client

clean:
	rm -rf crypto-{server,client} echo-{server,client,load-client}
