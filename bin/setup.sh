#!/bin/bash

rm -f buildpack.zip

zip -r buildpack.zip .

cf create-buildpack spire-agent-sidecar_buildpack buildpack.zip 1

rm -f buildpack.zip