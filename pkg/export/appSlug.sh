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

# 定义当前服务组件的名字
APPNAME=$(basename $(pwd))

# 定义当前工作目录
HOME=$(pwd)


# 解压 slug 包
function processSlug() {
    if [ -f ${APPNAME}-slug.tgz ]; then
        tar xzf ${APPNAME}-slug.tgz -C $HOME
    else
        echo -e "There is no slug file, ${0#*/} need it to start your app ...$RED Failure $NC"
        exit 1
    fi
}

# 运行 .profile.d 中的所有文件
# 这个过程会修改 PATH 环境变量
function processRuntimeEnv() {
    sleep 1
    if [ -d .profile.d ]; then
        echo -e "Handling runtime environment ... $GREEN Done $NC"
        for file in .profile.d/*; do
            source $file
        done
        hash -r
    fi
}

# 导入用户自定义的其他环境变量
function processCustomEnv() {
    if [ -f ${APPNAME}.env ]; then
        sleep 1
        echo -e "Handling custom environment ... $GREEN Done $NC"
        source ${APPNAME}.env
    fi
}

# 处理启动命令
function processCmd() {
    # 从 Procfile 文件中截取
    if [ -f Procfile ]; then
        # 渲染启动命令中的环境变量
        eval "cat <<EOF
$(<Procfile)
EOF
" >${APPNAME}.cmd
        sed -i 's/web: //' ${APPNAME}.cmd
    elif [ ! -f Procfile ] && [ -s .release ]; then
        eval "cat <<EOF
$(cat .release | grep web | sed 's/web: //')
EOF
" >${APPNAME}.cmd
    else
        echo -e "Can not detect start cmd, please check whether file Procfile or .release exists ... $RED Failure $NC"
        exit 1
    fi
}

# 启动函数
function appStart() {
    appStatus >/dev/null 2>&1 &&
        echo -e "App ${APPNAME} is already running with pid $(cat ${APPNAME}.pid). Try exec $0 status" &&
        exit 1
    processSlug
    processRuntimeEnv
    processCustomEnv
    processCmd
    echo "Running app ${APPNAME}, you can check the logs in file ${APPNAME}.log"
    echo "We will start your app with ==> $(cat ${APPNAME}.cmd)"
    nohup $(cat ${APPNAME}.cmd) >${APPNAME}.log 2>&1 &
    # 对于进程运行过程中报错退出的，需要时间窗口来延迟检测
    sleep 3
    # 查询进程，来确定是否启动成功
    RES=$(ps -p $! -o pid= -o comm=)
    if [ ! -z "$RES" ]; then
        echo -e "Running app ${APPNAME} with process: $RES ... $GREEN Done $NC"
        echo $! >${APPNAME}.pid
    else
        echo -e "Running app ${APPNAME} failed,check ${APPNAME}.log ... $RED Failure $NC"
    fi
}

function appStop() {
    if [ -f ${APPNAME}.pid ]; then
        PID=$(cat ${APPNAME}.pid)
        if [ ! -z $PID ]; then
            # For stopping Nginx process,SIGTERM is better than SIGKILL
            kill -15 $PID >/dev/null 2>&1
            if [ $? == 0 ]; then
                echo -e "Stopping app ${APPNAME} which running with pid ${PID} ... $GREEN Done $NC"
                rm -rf ${APPNAME}.pid
            else
                rm -rf ${APPNAME}.pid
            fi
        fi
    else
        echo "The app ${APPNAME} is not running.Ignore the operation."
    fi
}

# # TODO
# function appRestart() {

# }

# 获取当前目录下的 app 是否启动
function appStatus() {
    PID=$(cat ${APPNAME}.pid 2>/dev/null)
    RES=$(ps -p $PID -o pid= -o comm= 2>/dev/null)
    if [ ! -z "$RES" ]; then
        printf "%-30s %-30s %-10s\n" AppName Status PID
        printf "%-30s \e[1;32m%-30s\e[m %-30s\n %-10s\n" ${APPNAME} "Active(Running)" $PID
        return 0
    else
        printf "%-30s %-30s %-30s\n" AppName Status PID
        printf "%-30s \e[1;31m%-30s\e[m %-30s\n" "${APPNAME}" "Inactive(Exited)" "N/A"
        return 1
    fi
}

function showHelp() {
    sed -rn -e "s/^### ?//p" $0 | sed "s#app.sh#${0}#g"
}

case $1 in
start)
    appStart
    ;;
stop)
    appStop
    ;;
status)
    appStatus
    ;;
*)
    showHelp
    exit 1
    ;;
esac
