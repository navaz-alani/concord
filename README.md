# Concord Packet Protocol (CPP)

This document provides the definition of the Concord Packet Protocol (henceforth
referred to as CPP, or "the Protocol").

__NOTE__: [This GitHub Repository](https://github.com/navaz-alani/concord)
provides an implementation of the Protocol, currently only for Golang. However,
the protocol is language agnostic and can therefore be implemented in multiple
languages. The Protocol attempts to be relatively simple and easily
implementable in multiple languages. Furthermore, CPP's definition is such that
applications may easily create their own packet types, for example for
efficiency purposes.

## Introduction

CPP is an application layer protocol, with an interface similar to HTTP. CPP,
like HTTP, is a client-server protocol and it defines a Packet entity and how
servers should process these packets.

## Packets

The centerpiece of CPP is the Packet entity. It is the object which is
transmitted between clients and servers. Packets have two main sections/parts:

* Metadata - this is (not exclusively) Protocol-related data. Metadata
  exists in a "map" form i.e. in string key-value pairs. The Protocol defines
  certain metadata keys which are useful.
* Data - this is the section of the packet which is for use by the application.
  There is no specification on the structure of this data section and
  applications may use this section in any way.

Here are the Protocol specified packet metadata keys:

* `KeyTarget` is the string `"_tgt"`. It specifies the server target invoked by
  the packet (explained in the next section).
* `KeySvrStatus` is the string `"_stat"`. This is on response packets from a
  server, indicating the status of the request. If this metadata key is defined,
  it possibly has the value "-1", which indicates a server side processing
  error.
* `KeySvrMsg` is the string `"_msg"`. It holds an server error message, when
  defined.
* `KeyRef` is the string `"_ref"`. It is a unique identifier sent by a client.
  When this key is defined on a packet by a client, the server ensures that it
  is present on the response packet.

There is also a server function called "relay" which is used to send packets to
other addresses. This requires metadata keys and they are:

* `KeyRelayFrom` is the string `"_relay_src"`. It specifies the relayer's
  address.
* `KeyRelayTo` is the string `"_relay_dst"`. It specifies the address to which
  the packet is to be relayed.

## Servers

Servers are the next key component in CPP. A CPP server does two things:
processes packets sent by clients and (possibly) sends responses to these
packets. Servers in CPP define "targets" to which clients can direct packets
for processing (this is somewhat analogous to HTTP endpoints). Formally, a
"target" is a sequence of callbacks which operate on a packet under a shared
context. This will be exponded on more in the next subsection.

We now focus on how servers process packets.

### Server Side Processing

There are two stages of server side packet processing: the data and the packet
stages. In each of these stages, processing is similar and is based on the idea
of pipelines. The data stage uses "Data Pipelines" while the packet stage uses
"Packet Pipelines", both of which are conceptually exactly the same. A
Data/Packet pipeline is a series of callbacks which operate on a particular
piece of data (binary/packet respectively) under a shared context. The "shared
context" for processing mentioned here is used by the pipeline to modify the
behaviour of the server through status codes. Here are the currently defined
context status codes:

* `CodeContinue` means "continue pipeline execution". It indicates that the
  current pipeline stage succeeded.
* `CodeStopError` means "stop pipeline execution and return error". It indicates
  that the current pipeline stage encountered an irrecoverable error and the
  processing cannot continue. The server should then respond with an error
  message (which is also contained in the context).
* `CodeStopCloseSend` means "stop pipeline execution and continue with
  processing". It indicates that the current pipeline should be prematurely
  stopped, but not due to an error.
* `CodeStopNoop` means "stop pipeline execution and do nothing". It indicates
  that the current pipeline should be prematurely stopped (possibly due to an
  error) and server should not do anything else related to the data/packet being
  processed. So if the pipeline is a Packet Pipeline, this code would force the
  server to stop processing the packet without sending a response to the sender.
* `CodeRelay` means "stop pipeline execution and send the response to another
  address". It indicates that pipeline execution should be stopped and the
  response should be sent to another address, specified in the response packet's
  metadata (under the server relay keys).

The notion of these Data/Packet pipelines is what makes CPP servers easily
extensible. An example of this is the Cryptographic extension which easily
provides the ability to secure client-server communication by installing key
exchange targets on the server and encryption/decryption stages in the Data
pipeline.

#### Data Processing Stage

__NOTE__: In the following, "the wire" refers to the underlying connection, for
example a UDP socket/TCP connection.

The data stage is more low-level and allows for operations on the binary
representation of a packet. This stage occurs at two points: when the packet's
binary data is read off the wire and before it is written to the wire.

A great application of this is transport layer encryption (analogous to TLS, but
admittedly much simpler). The Cryptographic extension, which is part of the
Protocol specification does exactly this - it will be discussed after server
size processing stages have been discussed.

A server has exactly two Data pipelines, `DATA_IN` and `DATA_OUT`, which are
used for all data read and written to the wire respectively (note that these
names do not matter in server implementations as they will be defined as
constants). Data pipelines operate on the binary data and return a modified form
of that data for the next stage in the pipeline to operate on. After the Data
pipeline execution has been compelted, the final binary data will be decoded
into a packet (in the case of the `DATA_IN` pipeline) or written to the wire (in
the case of the `DATA_OUT` pipeline). Of course, the context status codes can be
used to modify the server's behaviour.

#### Packet Processing Stage

After the `DATA_IN` pipeline execution has completed successfully, the server
then decodes the binary data from the pipeline into a packet, which will be
operated on under a Packet pipeline. However, there are cases where this
pipeline execution may not succeeed:

* If the decoding of the binary data into a packet fails
* If the target specified by the packet is invalid

The second of these points is important. Every decoded packet needs to specify
the Packet pipeline which operates on it. This is where server targets come in.
A packet's metadata contains the server target that it invokes - this is under
the `KeyTarget` metadata key. A server contains Packet pipelines mapped to
targets and when a packet has been decoded, it is operated on by the pipeine
corresponding to its target. The Packet pipeline is a bit different from the
Data pipeline in that each packet pipeline stage appends data/sets metadata on
the response packet, rather than returning a response packet. After the Packet
pipeline is executed, the response packet is then converted to binary, passed
through the `DATA_OUT` pipeline and sent over the wire. The recipient of the
response packet is not always the sender of the packet. If the application
decides that the response should be sent to another address, it can specify so
in the Packet pipeline context status using `CodeRelay` and then supply the
relay address in the response packet's `KeyRelayTo` metadata key (if `CodeRelay`
is supplied and no address is provided in `KeyRelayTo`, the behaviour of the
server is indistinguishable from the case where the context code `CodeStopNoop`
is supplied).


### Extending Server Capabilities (and the `Crypto` Extension)

Using the Data/Packet pipeline architecture, extensions to servers are easy to
craft. For example, the `Crypto` extension is a Protocol specified server
extension (which also has client handles). It aims to make two things possible -
transport layer encryption and end-to-end packet data encryption (which makes
sense in packet relay cases).

__Crypto Specs__: Currently, the `Crypto` extension uses Elliptic Curve Diffie
Hellman (ECDH) for secret key generation, using the NIST-P256 elliptic curve
parameters. Encryption is then performed using the secret key and AES.

The `Crypto` extension firstly maintains a list of public keys of clients which
have performed a key-exchange with the server. To make the key-exchange
possible, the extension installs two key-exchange targets onto the server:

* Firstly, there is the `TargetKeyExchangeServer` target (which is the string
  `"crypto.kex-cs"`). This target expects the client's public key in JSON
  format (conatining only the `(x,y)` point on the NIST-P256 curve). Here is the
  format for the public key used by the `Crypto` extension:
  ```JSON
  { X: "<x-point>", Y: "<y-point>" }
  ```
  The server will then respond with its public key in the same format. A shared
  key is then established using ECDH and all further communication is AES
  encrypted using that shared key.
* Secondly, there is a `TargetKeyExchangeClient` target (which is the string
  `"crypto.kex-cc"`). This target expects the IP address of the client whose key
  to obtain, in JSON format. Here is the format for the request:
  ```JSON
  { ip: "<other-ip>" }
  ```
  If the other client has not performed a key-exchange with the server or the
  packet data failed to decode, the response packet contains an error message
  specifying this. Otherwise, the response packet contains the public key (in
  the previously specified format) of the other client. This key is used to
  generate a secret shared key with the other client using ECDH. The `Crypto`
  extension provides clients functions for end-to-end encrypting packet data for
  packets destined to the other client, using this shared key. Then other client
  also needs to obtain the public key of the client before they can decode
  end-to-end encrypted packets relayed by the client.

On the server, the `Crypto` extension installs callbacks onto the `DATA_IN` and
`DATA_OUT` pipelines to perform transport layer encryption/decryption
respectively if the sender/recipient respectively has been key-exchanged with.
