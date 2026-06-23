FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app ./cmd/worker

# Compile ffmpeg ourselves: we need both the non-free libfdk_aac encoder (the audio
# worker panics at startup without it, see cmd/worker/main.go) AND the hap encoder
# (--enable-libsnappy). No publicly distributable prebuilt ffmpeg ships both.
#
# To cover as broad a codec/format spectrum as possible without hand-maintaining a
# huge flag list, we mirror Ubuntu's own ffmpeg library set (via `apt build-dep` +
# the --enable-lib* flags from Ubuntu's packaged ffmpeg) and add --enable-nonfree
# --enable-libfdk-aac --enable-libsnappy on top. Only ffmpeg itself is compiled; all
# dependency libraries come prebuilt from apt. Hardware backends (nvenc/vaapi/qsv)
# are intentionally excluded -- they are not --enable-lib* flags and the CPU workers
# have no GPU.
FROM ubuntu:24.04 AS ffmpeg-build
ARG FFMPEG_VERSION=8.0.3
# Enable universe + multiverse + the deb-src repos that `apt build-dep` requires.
RUN apt-get update && apt-get install -y --no-install-recommends \
      software-properties-common ca-certificates curl xz-utils \
 && add-apt-repository -y universe \
 && add-apt-repository -y multiverse \
 && sed -i -E 's/^Types: deb$/Types: deb deb-src/' /etc/apt/sources.list.d/ubuntu.sources \
 && apt-get update
# Toolchain + the full Ubuntu ffmpeg build-dependency set + our non-free/hap extras.
RUN apt-get build-dep -y ffmpeg \
 && apt-get install -y --no-install-recommends \
      build-essential pkg-config nasm yasm \
      libfdk-aac-dev libsnappy-dev
# Fetch ffmpeg source.
RUN curl -fsSL "https://ffmpeg.org/releases/ffmpeg-${FFMPEG_VERSION}.tar.xz" -o /tmp/ffmpeg.tar.xz \
 && mkdir -p /ffmpeg-src && tar xf /tmp/ffmpeg.tar.xz -C /ffmpeg-src --strip-components=1
WORKDIR /ffmpeg-src
# Take Ubuntu's exact set of external-library enables (broad spectrum), drop fdk/snappy
# so we control those, and add our non-free fdk-aac + snappy (hap) on top.
RUN apt-get install -y --no-install-recommends ffmpeg \
 && LIBFLAGS="$(ffmpeg -hide_banner -version \
        | sed -n 's/^ *configuration: //p' \
        | tr ' ' '\n' \
        | grep -E '^--enable-lib' \
        | grep -vE 'fdk|snappy' \
        | tr '\n' ' ')" \
 && echo "Mirroring Ubuntu library flags: ${LIBFLAGS}" \
 && ./configure --prefix=/usr/local \
      --enable-gpl --enable-version3 --enable-nonfree \
      ${LIBFLAGS} \
      --enable-libfdk-aac --enable-libsnappy \
      --disable-doc --disable-ffplay \
 && make -j"$(nproc)" && make install
# Stage exactly the shared libraries the two binaries link (the full transitive
# closure ldd resolves), copying the real files with `cp -L` so the ldconfig
# symlink names (e.g. libfoo.so.11) the loader needs are preserved as real files.
# Normalize /lib -> /usr/lib so everything lands under /usr: on the runtime base
# /lib is a symlink to /usr/lib (usrmerge), and COPY refuses dir-onto-symlink.
RUN mkdir -p /deps \
 && ldd /usr/local/bin/ffmpeg /usr/local/bin/ffprobe \
      | awk '/=> \//{print $3}' | sort -u \
      | while read -r lib; do \
            dest="/deps$(dirname "$lib" | sed 's#^/lib/#/usr/lib/#')"; \
            mkdir -p "$dest"; cp -L "$lib" "$dest/"; \
        done

# Lean runtime: just the two binaries, the .so files they actually link, fonts for
# drawtext/libass, and ca-certificates. No ffmpeg metapackage, no duplicate binary.
FROM ubuntu:24.04 AS runtime
RUN apt-get update && apt-get install -y --no-install-recommends \
      fonts-dejavu-core ca-certificates \
 && rm -rf /var/lib/apt/lists/*
COPY --from=ffmpeg-build /deps/ /
COPY --from=ffmpeg-build /usr/local/bin/ffmpeg  /usr/local/bin/ffmpeg
COPY --from=ffmpeg-build /usr/local/bin/ffprobe /usr/local/bin/ffprobe
RUN ldconfig
WORKDIR /
COPY --from=build /app /worker/bin
ENTRYPOINT ["/worker/bin"]
