FROM ubuntu:24.04@sha256:224a1869083a311ef3f13648a154ba79832fbef6364d31493642ca03082da254
ENV container=docker
ENV DEBIAN_FRONTEND=noninteractive TZ=Etc/UTC
RUN apt-get update \
    && apt-get install -y --no-install-recommends curl dbus libcap2-bin python3 systemd systemd-sysv \
    && rm -rf /var/lib/apt/lists/*
STOPSIGNAL SIGRTMIN+3
# Root PID 1 is required only for disposable lifecycle/containment tests.
CMD ["/sbin/init"]
