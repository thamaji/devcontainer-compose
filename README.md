devcontainer-compose
====

VSCode Remote Container で devcontainer の中で docker-compose を使うときの volumes の問題をなんとなく解決します。

devcontainer 内で docker を使うために `/var/run/docker.sock` をマウントしていると、docker-compose.yml の volumes で指定したパスが devcontainer 内のパスではなくホストのパスとして解釈されます。

このツールは docker-compose コマンドをラップし、volumes のパスを devcontainer 内のパスとして解釈させるものです。

## Usage

バイナリをコピーして、本家の docker-compose よりも優先されるように PATH を設定します。

たとえば、Dockerfile に以下のように記述します。

```
RUN set -x \
    && mkdir -p /usr/local/devcontainer-tool/bin \
    && curl -fsSL -o /usr/local/devcontainer-tool/bin/docker-compose https://github.com/thamaji/devcontainer-compose/releases/download/v1.0.2/docker-compose \
    && chmod +x /usr/local/devcontainer-tool/bin/docker-compose
ENV PATH=/usr/local/devcontainer-tool/bin:${PATH}
```
