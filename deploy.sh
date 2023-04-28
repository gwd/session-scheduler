#!/bin/bash

SESSIONSDIR="session-scheduler-test"
if [[ -n "$1" ]] ; then
    SESSIONSDIR="$1"
fi

HOST=xensched@xenbits.xenproject.org

rsync -rvz assets templates session-scheduler $HOST:$SESSIONSDIR/
ssh $HOST "mkdir -p $SESSIONSDIR/data"
