#!/bin/bash

set -e

export GOPATH="$(pwd)/_gopath"
export GOCACHE="$(pwd)/_gocache"
export GOBIN="$(pwd)/bin"

if [ -n "${GO_PACKAGE}" ] && [ "$GO111MODULES" = off ]; then
  if ! [ -d _gopath ]; then
    tempgopath="$(mktemp -d)"
    pkgroot="${GO_PACKAGE%/*}"

    mkdir -p "${tempgopath}/src/${pkgroot}"
    cp -a . "${tempgopath}/src/${GO_PACKAGE}"
    mv "${tempgopath}" "${GOPATH}"

    unset pkgroot
    unset pkgpath
    unset tempgopath
  fi

  cd "${GOPATH}/src/${GO_PACKAGE}"
fi

echo "# go${*:+ }$*" | sed 'p;s/./-/g'
exec go "$@"
