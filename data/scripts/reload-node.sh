#!/bin/sh

# Reload environment variables from the env_file
export $(grep -v '^#' /data/env_file | xargs)

# Start the main application
exec "$@"
