#!/bin/bash
set -euxo pipefail

mkdir -p output

latexmk \
  -pdfxe \
  -output-directory=output \
  *.tex

latexmk \
  -c \
  -output-directory=output \
  *.tex
