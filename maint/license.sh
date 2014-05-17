#!/bin/bash
for f in $(find . -type f -name '*.go'); do
  if ! grep -q Copyright "$f"; then
    cat maint/copyright.txt "$f" >"$f.new" && mv "$f.new" "$f";
  fi
done
