#!/bin/bash

SESSIONSDIR="session-scheduler"

rsync -rvz assets templates session-scheduler run.sh xensched@xenbits:$SESSIONSDIR/
ssh xensched@xenbits "mkdir -p $SESSIONSDIR/data"
