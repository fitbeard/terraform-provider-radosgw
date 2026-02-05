FROM mcr.microsoft.com/devcontainers/base:ubuntu22.04

ARG CEPH_RELEASE

RUN curl -fsSL https://download.ceph.com/keys/release.asc | gpg --dearmor -o /usr/share/keyrings/ceph-archive-keyring.gpg && \
    echo "deb [signed-by=/usr/share/keyrings/ceph-archive-keyring.gpg] https://download.ceph.com/debian-${CEPH_RELEASE}/ jammy main" > /etc/apt/sources.list.d/ceph.list

RUN apt-get update && apt-get install --yes \
    ceph \
    ceph-mgr-dashboard \
    python3-setuptools \
    radosgw \
    unzip \
    wget \
    git \
    jq \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

# Workaround for https://tracker.ceph.com/issues/74199
RUN rm -fr /usr/share/ceph/mgr/dashboard/controllers/smb.py

# Persist bash history.
RUN mkdir /command-history \
    && touch /command-history/.bash_history \
    && chown -R vscode /command-history

ENV PROMPT_COMMAND='history -a'
ENV HISTFILE=/command-history/.bash_history
ENV HISTSIZE=10000

RUN mkdir -p /workspaces /external-files && \
    chown -R vscode /workspaces /external-files

USER vscode
WORKDIR /workspaces
