#!/bin/sh
# Fetches the real GLEIF Root, GLEIF External, Provenant QVI and GLEIF Internal key histories
# from their witnesses into <dir>/{root,ext,prov,internal}.cesr (first witness that answers).
# Usage: script/fetch_real.sh <dir>     then: python -I script/extract.py <dir>
set -eu
dir=$1
mkdir -p "$dir"
ROOT=EDP1vHcw_wc4M__Fj53-cJaBnZZASd-aMTaSyWEQ-PC2
EXT=EINmHd5g7iV-UldkkkKyBIH052bIyxZNBn9pq-zNrYoS
PROV=ED88Jn6CnWpNbSYz6vp9DOSpJH2_Di5MSwWTf1l34JJm
INT=EFcrtYzHx11TElxDmEDx355zm7nJhbmdcIluw7UMbUIL
GLEIF="5.161.69.25 65.21.253.212 8.210.213.186 51.79.54.121 102.37.159.99 54.233.109.129 13.244.119.106 51.161.130.60 20.78.61.227 8.208.27.153"

get() { # name prefix url...
  name=$1 pre=$2; shift 2
  for base in "$@"; do
    if curl -fsS --max-time 60 -o "$dir/$name.cesr" "$base/oobi/$pre/witness" && grep -q "\"i\":\"$pre\"" "$dir/$name.cesr"; then
      echo "$name <- $base ($(wc -c <"$dir/$name.cesr") B)"
      return
    fi
  done
  echo "$name: no witness answered" >&2
  exit 1
}

get root $ROOT $(for h in $GLEIF; do echo "http://$h:5623"; done)
get ext $EXT $(for h in $GLEIF; do echo "http://$h:5623"; done)
get prov $PROV $(for n in 1 2 3 4 5; do echo "http://witness$n.main.provenant.net:5631"; done)
get internal $INT $(for h in $GLEIF; do echo "http://$h:5623"; done)
