#!/bin/sh

# binwalk -e --dd="jpeg:jpg" 0I-aulas.pdf
montage \
    -geometry 3136 \
    -tile x2 \
    -gravity North \
    "_0I-aulas.pdf.extracted/A3.jpg" \
    "_0I-aulas.pdf.extracted/6737C.jpg" \
    "aulas.jpg"
