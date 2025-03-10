#!/bin/sh

calmailproc="go run main.go"

args="-store -vdir test/vdir"

for file in test/mails/*.eml; do
    echo $file
    $calmailproc $args < "$file"
done
