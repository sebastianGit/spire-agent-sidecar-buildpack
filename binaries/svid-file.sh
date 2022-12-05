#!/bin/bash

while (true)
do
  echo "Refresh SVID"
  ./spire-agent api fetch x509 -socketPath /tmp/spire-agent/public/api.sock -write /tmp/spire-agent
  sleep 1000
done