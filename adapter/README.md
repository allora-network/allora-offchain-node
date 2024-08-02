# Adapters

This directory contains adapters for configuring different options in the node.

## Adding an adapter

To add an adapter: 
* Add a directory and corresponds to the package name.
* create a main.go file inside the package implementing the interface `lib.AlloraAdapter`.
* add a case in the switch in `adapter_factory.go`.
