#!/bin/bash
set -euxo pipefail

unzip input.zip

mkdir -p output

xelatex -halt-on-error -output-directory=output *.tex
xelatex -halt-on-error -output-directory=output *.tex
