#!/bin/bash

# Set up the development container for the Orbit project (for Github Codespaces)
# This is the `postCreateCommand` for the .devcontainer/devcontainer.json file 

echo "Setting up development container for Orbit project..."
sudo apt-get update

echo "Installing SQLite3 and its development libraries..."
sudo apt-get install -y sqlite3 libsqlite3-dev 

echo "Installing Python dependencies using UV..."
uv sync
