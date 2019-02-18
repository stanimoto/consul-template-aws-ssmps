#!/bin/sh -x

set -e
set -u

VERSION=$(gobump show -r)
VERSION_DIR="dist/v$VERSION"

goxz -d $VERSION_DIR -n ssmps -o ssmps -os "linux,darwin" -arch "amd64,386"
(cd $VERSION_DIR && shasum -a 256 * > v${VERSION}_SHASUMS)
