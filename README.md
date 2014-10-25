psmcli
======

This program implements a simple command line client to the PSM JSON-RPC
interface. It's primarily useful for exploration, debugging and making
small changes interactively.

psmcli is an open source component provided without any warranty or
support. Please read the LICENSE file.

Supported Features
------------------

 * All PSM JSON-RPC commands.

 * Tab completion of commands and object types.

 * Parameter value hinting (using tab completion).

 * JSON objects as parameters, using key1=val,key2=val syntax.

 * Authentication, when required by PSM.

 * Printing of the actual executed JSON-RPC command (when run with the
   -v flag.)

Requirements
------------

A configured and enabled JSON-RPC source.

Example
-----

```$bash
$ psmcli psm.example.com:3994
psmcli 1.0.0 connected to 172.16.32.8:3994
PSM version 15.0.5.12 at psm.example.com

default@psm # subscriber list 1
{
    "creationTime": "2014-09-15T08:17:08.623Z",
    "expire": "2014-11-01T10:47:16.207Z",
    "hostName": "syno-2",
    "oid": 288230376151715606,
    "parentOid": null,
    "persistent": true,
    "slot": 80,
    "subscriberId": "20:c9:d0:43:0c:95",
    "updateTime": "2014-10-25T10:47:16.207Z"
}

default@psm # object updateByAid subscriber 20:c9:d0:43:0c:95 hostName=newHostName,persistent=false
OK
default@psm # subscriber getByUid 288230376151715606
{
    "creationTime": "2014-09-15T08:17:08.623Z",
    "expire": "2014-11-01T10:47:16.207Z",
    "hostName": "newHostName",
    "oid": 288230376151715606,
    "parentOid": null,
    "persistent": false,
    "slot": 80,
    "subscriberId": "20:c9:d0:43:0c:95",
    "updateTime": "2014-10-25T10:56:44.049Z"
}

default@psm #
```
