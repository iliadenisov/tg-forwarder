#!/bin/sh

echo "Running app"

/bin/sh -c $@
exit_code=$?

if [ $exit_code -ne 0 ]; then
    echo "App terminated with an error; pausing for 10 second before exit..."
    sleep 10s
fi

echo "App exit code: ${exit_code}"
exit ${exit_code}
