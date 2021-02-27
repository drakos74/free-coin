#!/bin/bash

cd /home/ec2-user/Tools/grafana-7.4.3

dir=$(pwd)
echo "grafana directory '$dir'"

pgrep "grafana" | xargs kill
echo "killed grafana"

./bin/grafana-server web >grafana.log 2>&1 &
echo "started grafana"
