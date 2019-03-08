#!/bin/bash
UPDATE_DIR=$1
if [[ -z "$UPDATE_DIR" ]] ; then
    echo "Usage: $0 bootstrap_dir"
    exit 1
fi

for f in $((find assets/ -type f -name "bootstrap*") | sed "s/assets\///"); do
    echo "Updating $f"
    cp $UPDATE_DIR/$f assets/$f
done
