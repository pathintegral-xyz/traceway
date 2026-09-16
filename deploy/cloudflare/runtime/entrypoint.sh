#!/bin/sh
set +e

/usr/local/bin/traceway > /tmp/traceway-startup.log 2>&1
status=$?
cat /tmp/traceway-startup.log
sleep 300
exit "$status"
