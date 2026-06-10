FROM ubuntu:24.04

# Only apt-related env here — changing anything above the waydroid layer
# busts its cache and triggers a multi-GB re-download.
ENV DEBIAN_FRONTEND=noninteractive

# Layer 1 — waydroid (slow, changes rarely — must stay cached)
RUN apt-get update && \
    apt-get install -y --no-install-recommends curl ca-certificates && \
    curl https://repo.waydro.id | bash && \
    apt-get install -y --no-install-recommends waydroid && \
    rm -rf /var/lib/apt/lists/*

# Layer 2 — display stack (faster, safe to modify without busting waydroid cache)
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        python3-pip \
        wl-clipboard \
        dbus \
        cage \
        weston && \
    rm -rf /var/lib/apt/lists/*

# Runtime env — after all slow layers so changing these doesn't bust cache
ENV WLR_BACKENDS=x11 \
    WLR_RENDERER=pixman \
    XDG_SESSION_TYPE=x11

RUN printf '#!/bin/sh\nexec true\n' > /usr/local/bin/modprobe && \
    chmod +x /usr/local/bin/modprobe

COPY internal/container/entrypoint.sh /usr/bin/waydroid-session.sh
RUN chmod +x /usr/bin/waydroid-session.sh

ENTRYPOINT ["/usr/bin/waydroid-session.sh"]
