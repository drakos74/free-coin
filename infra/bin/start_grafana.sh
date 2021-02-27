#!/bin/bash

pgrep "grafana" | xargs kill
./bin/grafana-server web >grafana.log 2>&1 &