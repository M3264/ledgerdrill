#!/bin/sh
case " ${RENEWED_DOMAINS:-} " in
  *" ld.kennyy.tech "*)
    /bin/systemctl reload nginx
    ;;
esac
