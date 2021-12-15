# memhttp

This is a HTTP server built to serve all its content from memory.

## Installation

You need to have Go installed. You can install the server by go get'ing it.

    go get github.com/satran/memhttp


## Configuration

Configuration is done using environment variables. These are the variables that can be set:

- `HOSTNAME`: set the hostname
- `CERT` & `KEY`: if you intend on using TLS, the files for the certificate
- `SITE`: the directory from which to serve the site from
- `ALIAS`: json file defining the path aliases

