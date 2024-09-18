#!/bin/bash

for i in {1..20}; do
  gh workflow run go-test.yml --ref=feature/without_gorm
  sleep 120
done

