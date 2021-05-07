#!/bin/bash

SESSIONSDIR="session-scheduler"
if [[ -n "$1" ]] ; then
    SESSIONSDIR="$1"
fi

rsync -rvz assets templates session-scheduler xensched@xenbits:$SESSIONSDIR/
ssh xensched@xenbits "mkdir -p $SESSIONSDIR/data"
