# Adapters

This directory contains adapters for configuring different options in the node.

## Adding an adapter

To add an adapter: 
* Add a directory that corresponds to the type eg API, Postgres, etc 
* `cd` into the directory and add another directory that corresponds to the package name.
* You can also add your source (eg API server, Postgres db, etc) into this directory
* Create a main.go file inside the package implementing the interface `lib.AlloraAdapter`.
* add a case in the switch in `adapter_factory.go`.
