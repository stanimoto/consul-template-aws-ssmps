#!/bin/sh -x

set -e
set -u

for GOOS in darwin
do
    for GOARCH in 386 amd64
    do
        mkdir -p "dist/$GOOS-$GOARCH"
        GOOS="$GOOS" GOARCH="$GOARCH" go build -o "dist/$GOOS-$GOARCH/ssmps"
    done
done

for GOOS in linux
do
    for GOARCH in 386 amd64 arm
    do
        mkdir -p "dist/$GOOS-$GOARCH"
        GOOS="$GOOS" GOARCH="$GOARCH" go build -o "dist/$GOOS-$GOARCH/ssmps"
    done
done
