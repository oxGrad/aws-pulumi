mod scripts
mod bootstrap
mod platform

_default:
  @just --choose

build-lambda-notifier:
  ./platform/build-lambda.sh

