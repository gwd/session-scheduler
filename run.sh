#!/bin/bash

cd /home/xensched/session-scheduler

echo Running session scheduler

(with-lock-ex -q lock ./session-scheduler) 2>> session-scheduler.log
