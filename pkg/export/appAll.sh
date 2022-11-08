#!/bin/bash
###
### app.sh — Controls app startup and stop.
###
### Usage:
###   app.sh <Options>
###
### Options:
###   start   Start your app.
###   stop    Stop your app.
###   status  Show app status.
###   -h      Show this message.

[ $DEBUG ] && set -x

# make stdout colorful
GREEN='\033[1;32m'
YELLOW='\033[1;33m'
RED='\033[1;31m'
NC='\033[0m' # No Color

# 定义当前应用的名字
APPNAME=$(basename $(pwd))

# 扫描当前应用中所有的服务组件名称
APPS=$(ls -d */ | sed "s#\/##g")

# 启动所有的服务组件
function allAppStart() {
    for app in ${APPS}; do
        pushd $app >/dev/null 2>&1
        if [ ! -f ./$app.sh ]; then
            popd >/dev/null 2>&1
            continue
        fi
        ./$app.sh start | sed -n '$p'
        popd >/dev/null 2>&1
    done
}

function allAppStop() {
    for app in ${APPS}; do
        pushd $app >/dev/null 2>&1
        if [ ! -f ./$app.sh ]; then
            popd >/dev/null 2>&1
            continue
        fi
        ./$app.sh stop
        popd >/dev/null 2>&1
    done
}

function allAppStatus() {
    printf "%-30s %-30s %-10s\n" AppName Status PID
    for app in ${APPS}; do
        pushd $app >/dev/null 2>&1
        if [ ! -f ./$app.sh ]; then
            popd >/dev/null 2>&1
            continue
        fi
        ./$app.sh status | sed '1d'
        popd >/dev/null 2>&1
    done
}

case $1 in
start)
    allAppStart
    ;;
stop)
    allAppStop
    ;;
status)
    allAppStatus
    ;;
*)
    showHelp
    exit 1
    ;;
esac
