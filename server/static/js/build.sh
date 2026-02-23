#!/usr/bin/env bash
# shellcheck shell=bash

cd "$(dirname -- "$0")" || exit

npm install && npm run build
