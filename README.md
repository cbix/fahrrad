# fahrrad

This is fahrrad, the **F**ast **A**nd **H**ackable **R**edis-backed **R**outer **A**dvertisement **D**aemon made in germany. It's still under heavy development, so don't use it in production environments!

## Use cases
* IPv6 networks that require precise control over prefix assignment without using DHCPv6
* High availability routers with live configuration or frequent configuration changes

## Redis database
The database has to provide the following keys:
* `fahrrad/mac/{BINARY_MAC}` → `{BINARY_PREFIX}`: `{BINARY_MAC}` is the link-layer address
  of a client in binary form (i.e. `aa:bb:cc:dd:ee:ff` → `\xaa\xbb\xcc\xdd\xee\xff`) and
  `{BINARY_PREFIX}` the IPv6 prefix (/64) that this client should get advertised (i.e.
  `2001:db8::/64` → `\x20\x01\x0d\xb8\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00`)

## Build and run
    $ go get github.com/CBiX/fahrrad
    # $GOPATH/bin/fahrrad

## ref
* RFC 4862

