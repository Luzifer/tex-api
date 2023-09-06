#!/bin/bash
set -euxo pipefail

mkdir -p output

xelatex -halt-on-error -output-directory=output *.tex
xelatex -halt-on-error -output-directory=output *.tex
