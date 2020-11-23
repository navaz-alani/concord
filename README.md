# Concord Packet Protocol (CPP)

This is a simple application layer protocol called "Concord Packet
Protocol" (CPP). The repository contains an implementation of the protocol using
UDP as the transport layer protocol, with JSON for packets encoding. The
files in the `examples` directory demonstrate a client-server interaction using
CPP.

## Packets

Like other protocols for server-client communication, CPP works by sending and
receiving pieces of information, called __packets__. Packets have two main
components: __metadata__ and __data__. The metadata part of any packet contains
information, in the form of key-value pairs, used by the server and client to
perform "book-keeping" of packets. The data part of the packet is information
that the application layer is sending & receiving.

Servers in CPP are configured to offer a set of `target`s. A `target` can be
thought of as a function that the server offers. The packet sent by the client
is the input to this function and the output is the response sent to the client.
The function analogy is not exactly accurate. Actually, a target corresponds to
a "callback" queue of functions which have the same signature (`TargetCallback`
in the implementation) and perform the application layer work. The callback
functions in this queue are executed one by one (first to last) and each
performs some piece of work based on the received packet. Every time a target is
invoked by a packet, this callback queue is executed and each function runs
within a context shared by the other functions in the queue. This allows one
function to do some work for the next function in the queue, or use previously
done work. The shared context also makes it easy to short-circuit the processing
of a request from the application side.

Here are some important metadata keys that a packet may feature.

* The `ref` key is probably the most important key for a packet as it is its
  unique identifier. It is the responsibility of the client to set the `ref` key
  of a packet before sending it to the server. The server then maintains this
  `ref` key on its response. This allows clients to pair responses received with
  requests sent.
* The `target` key specifies the target that the packet should invoke when
  processed by the server.
* The `serverMsg` key is the server's message (if any).
* The `serverStatus` key contains the status code for the request. Usually, 0
  indicates that everything went well. Applications may internally use status
  keys.
