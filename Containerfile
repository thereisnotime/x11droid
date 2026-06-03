FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        curl \
        ca-certificates \
        python3-pip \
        wl-clipboard \
        cage \
        weston && \
    rm -rf /var/lib/apt/lists/*

RUN curl https://repo.waydro.id | bash && \
    apt-get install -y --no-install-recommends waydroid && \
    rm -rf /var/lib/apt/lists/*

RUN printf '#!/bin/bash\n\
COMPOSITOR="${WAYDROID_COMPOSITOR:-cage}"\n\
cleanup() {\n\
  trap - EXIT INT TERM HUP\n\
  waydroid session stop 2>/dev/null || true\n\
  killall waydroid cage weston 2>/dev/null || true\n\
}\n\
trap cleanup EXIT INT TERM HUP\n\
case "$COMPOSITOR" in\n\
  cage)\n\
    cage -s -- waydroid show-full-ui\n\
    ;;\n\
  weston)\n\
    weston --xwayland &\n\
    export WAYLAND_DISPLAY=wayland-0\n\
    sleep 2\n\
    waydroid show-full-ui\n\
    ;;\n\
  *)\n\
    echo "Unknown compositor: $COMPOSITOR" >&2\n\
    exit 1\n\
    ;;\n\
esac\n' > /usr/bin/waydroid-session.sh && \
    chmod +x /usr/bin/waydroid-session.sh

ENTRYPOINT ["/usr/bin/waydroid-session.sh"]
