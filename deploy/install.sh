#!/usr/bin/env bash

find . -name "*.yaml" | xargs -L1 kubectl apply -f 
