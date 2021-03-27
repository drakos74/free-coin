#!/bin/bash

increment_name() {
  name=$1

  cd ui/build/static/logs || exit

  if [[ -e $name.log || -L $name.log ]]; then
    i=1
    while [[ -e $name-$i.log || -L $name-$i.log ]]; do
      let i++
    done
    name=$name-$i
  fi
  full_name="$name".log

  echo "$full_name"
}

stop() {
  pgrep "$1" | xargs kill
}

build() {
  process="$1"
  cd cmd/"$process" || exit
  GOOS=linux GOARCH=amd64 go build -o "$process" -mod vendor
}

start() {
  process="$1"
  touch ui/build/static/logs/"$process".log
  ./cmd/"$process"/"$process" >ui/build/static/logs/"$process".log 2>&1 &
}

check() {
  process="$1"
  sleep 5
  if [[ $(pgrep "$process") ]]; then
    echo "$process running"
  else
    echo "error : could NOT run $process"
    exit 1
  fi
}

restart() {
  process="$1"

  dir=$(pwd)
  echo "current directory '$dir'"

  stop "$process"
  echo "stopped $process"

  build "$process"
  echo "built $process"

  cd "$dir" || exit

  new_name="$(increment_name "$process")"
  echo "move previous logs to '$new_name'"
  mv ui/build/static/logs/"$process".log ui/build/static/logs/"$new_name"
  cd "$dir" || exit

  start "$process"
  echo "starting $process"

  check "$process"
  echo "$process restarted!"
}


restart_grafana() {
  process="$1"

  dir=$(pwd)
  echo "current directory '$dir'"

  stop "$process"
  echo "stopped $process"

  bash /home/ec2-user/Tools/grafana-7.4.3/start_grafana.sh
  echo "starting $process"

  check "$process"
  echo "$process restarted!"
}

dir=$(pwd)
echo "$dir"

if [ ! "${dir: -9}" == "free-coin" ]; then
  echo "wrong directory '${dir: -9}' ! Make sure you run from 'free-coin' root"
  exit 1
fi

if [ $# -eq 0 ]
  then
    echo "No arguments supplied"
    # restart server process for ui and static file access
    restart "server"
    # restart coin process
    restart "coin"
    # restart backtesting process
    restart "backtest"
    # restart external process
    restart "external"
    # just stop and start
    restart_grafana "grafana"
    echo "ALL DONE!"
    exit 0
fi

i=1;
for process in "$@"
do
    echo "restarting $process"
    restart "$process";
    i=$((i + 1));
done
echo "ALL DONE!"
exit 0



